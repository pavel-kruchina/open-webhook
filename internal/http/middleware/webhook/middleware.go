package webhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"gh.tarampamp.am/webhook-tester/v2/internal/config"
	"gh.tarampamp.am/webhook-tester/v2/internal/files"
	"gh.tarampamp.am/webhook-tester/v2/internal/http/openapi"
	"gh.tarampamp.am/webhook-tester/v2/internal/pubsub"
	"gh.tarampamp.am/webhook-tester/v2/internal/storage"
)

func New( //nolint:funlen,gocognit,gocyclo
	appCtx context.Context,
	log *zap.Logger,
	db storage.Storage,
	fileStore files.Storage,
	pub pubsub.Publisher[pubsub.RequestEvent],
	cfg *config.AppSettings,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var sID, doIt = shouldCaptureRequest(r)
			if !doIt {
				next.ServeHTTP(w, r)

				return
			}

			// serve a stored file download: GET|HEAD /{session_uuid}/files/{file_uuid}
			// if the file is unknown, fall through and capture the request as usual
			if fID, isDownload := fileDownloadTarget(r, sID); isDownload {
				if serveFile(r.Context(), w, r, db, fileStore, log, sID, fID) { //nolint:contextcheck
					return
				}
			}

			var reqCtx = r.Context()

			// get the session from the storage
			sess, sErr := db.GetSession(reqCtx, sID) //nolint:contextcheck
			if sErr != nil {                         //nolint:nestif
				// if the session is not found
				if errors.Is(sErr, storage.ErrNotFound) {
					// but the auto-creation is enabled
					if cfg.AutoCreateSessions {
						// create a new session with some default values
						if _, err := db.NewSession(reqCtx, storage.Session{ //nolint:contextcheck
							Code: http.StatusOK,
						}, sID); err != nil {
							respondWithError(w, log, http.StatusInternalServerError, err.Error())

							return
						} else {
							// and try to get it again
							if sess, sErr = db.GetSession(reqCtx, sID); sErr != nil { //nolint:contextcheck
								respondWithError(w, log, http.StatusInternalServerError, sErr.Error())

								return
							} else {
								// add the header to indicate that the session has been created automatically
								w.Header().Set("X-Wh-Created-Automatically", "1")
							}
						}
					} else {
						respondWithError(w, log, http.StatusNotFound, "The webhook has not been created yet or may have expired")

						return
					}
				} else {
					respondWithError(w, log, http.StatusInternalServerError, sErr.Error())

					return
				}
			}

			{ // increase the session lifetime
				var delta = time.Now().Add(cfg.SessionTTL).Sub(time.Unix(0, sess.CreatedAtUnixMilli*int64(time.Millisecond)))

				if err := db.AddSessionTTL(reqCtx, sID, delta); err != nil { //nolint:contextcheck
					respondWithError(w, log, http.StatusInternalServerError, err.Error())

					return
				}
			}

			// read the request body
			var body []byte

			if r.Body != nil {
				if b, err := io.ReadAll(r.Body); err == nil {
					body = b
				}
			}

			// check the request body size and respond with an error if it's too large
			if cfg.MaxRequestBodySize > 0 && uint32(len(body)) > cfg.MaxRequestBodySize { //nolint:gosec
				respondWithError(w, log,
					http.StatusRequestEntityTooLarge,
					fmt.Sprintf("The request body is too large (current: %d, max: %d)", len(body), cfg.MaxRequestBodySize),
				)

				return
			}

			// if the request is a multipart/form-data upload, extract its files to the file storage and replace
			// the stored body with a sanitized version (form fields + file placeholders, without the file bytes)
			var capturedFiles []storage.RequestFile
			if ct := r.Header.Get("Content-Type"); ct != "" {
				if mediaType, params, mErr := mime.ParseMediaType(ct); mErr == nil && strings.HasPrefix(mediaType, "multipart/") {
					if boundary := params["boundary"]; boundary != "" {
						capturedFiles, body = extractMultipartFiles(reqCtx, fileStore, sID, body, boundary, log) //nolint:contextcheck
					}
				}
			}

			// convert request headers into the storage format
			var rHeaders = make([]storage.HttpHeader, 0, len(r.Header))
			for name, value := range r.Header {
				rHeaders = append(rHeaders, storage.HttpHeader{Name: name, Value: strings.Join(value, "; ")})
			}

			// sort headers by name
			slices.SortFunc(rHeaders, func(i, j storage.HttpHeader) int { return strings.Compare(i.Name, j.Name) })

			// and save the request to the storage
			rID, rErr := db.NewRequest(reqCtx, sID, storage.Request{ //nolint:contextcheck
				ClientAddr: extractRealIP(r),
				Method:     r.Method,
				Body:       body,
				Headers:    rHeaders,
				URL:        extractFullUrl(r),
				Files:      capturedFiles,
			})
			if rErr != nil {
				respondWithError(w, log, http.StatusInternalServerError, rErr.Error())

				return
			}

			w.Header().Set("X-Wh-Request-Id", rID)

			// publish the captured request to the pub/sub. important note - we should use the app ctx instead of the req ctx
			// because the request context can be canceled before the goroutine finishes (and moreover - before the
			// subscribers will receive the event - in this case the event will be lost)
			go func() {
				// read the actual data from the storage (the main point is the time of creation)
				captured, dbErr := db.GetRequest(appCtx, sID, rID)
				if dbErr != nil {
					log.Error("failed to get a captured request", zap.Error(dbErr))

					return
				}

				var headers = make([]pubsub.HttpHeader, len(captured.Headers))
				for i, h := range captured.Headers {
					headers[i] = pubsub.HttpHeader{Name: h.Name, Value: h.Value}
				}

				if err := pub.Publish(appCtx, sID, pubsub.RequestEvent{
					Action: pubsub.RequestActionCreate,
					Request: &pubsub.Request{
						ID:                 rID,
						ClientAddr:         captured.ClientAddr,
						Method:             captured.Method,
						Headers:            headers,
						URL:                captured.URL,
						CreatedAtUnixMilli: captured.CreatedAtUnixMilli,
					},
				}); err != nil {
					log.Error("failed to publish a captured request", zap.Error(err))
				}
			}()

			// wait for the delay if it's set
			if sess.Delay > 0 {
				sleep(reqCtx, sess.Delay) //nolint:contextcheck
			}

			// set the header to allow CORS requests from any origin and method
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "*")
			w.Header().Set("Access-Control-Allow-Headers", "*")

			// set the session headers
			for _, h := range sess.Headers {
				w.Header().Set(h.Name, h.Value)
			}

			// by default, use the status code from the session
			var statusCode = int(sess.Code)

			// extract requested status code from the request URL (it should be the last part)
			if parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/"); len(parts) > 1 {
				// loop over parts slice from the end to the beginning
				for i := len(parts) - 1; i >= 0; i-- {
					if code, err := strconv.Atoi(parts[i]); err == nil && code >= 100 && code <= 599 {
						statusCode = code

						break
					}
				}
			}

			// set the status code
			w.WriteHeader(statusCode)

			// write the response body
			if _, err := w.Write(sess.ResponseBody); err != nil { //nolint:gosec
				log.Error("failed to write the response body", zap.Error(err))
			}
		})
	}
}

// shouldCaptureRequest checks if the request should be captured (the path starts with a valid UUID).
func shouldCaptureRequest(r *http.Request) (string, bool) {
	if r.URL == nil {
		return "", false
	}

	var clean = strings.TrimLeft(r.URL.Path, "/")

	if len(clean) >= openapi.UUIDLength && openapi.IsValidUUID(clean[:openapi.UUIDLength]) {
		return clean[:openapi.UUIDLength], true
	}

	return "", false
}

// fileDownloadTarget reports whether the request targets a stored file download
// (GET|HEAD /{session_uuid}/files/{file_uuid}) and returns the requested file UUID.
func fileDownloadTarget(r *http.Request, sID string) (fileUUID string, _ bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return "", false
	}

	if r.URL == nil {
		return "", false
	}

	// expected path: <sID>/files/<fID>
	var parts = strings.Split(strings.TrimLeft(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != sID || parts[1] != "files" { //nolint:mnd
		return "", false
	}

	if !openapi.IsValidUUID(parts[2]) {
		return "", false
	}

	return parts[2], true
}

// serveFile streams a stored file to the client if it belongs to the given session. It returns true if the
// response was handled (served or a definitive error written), and false if the caller should fall through
// to the normal request-capture flow (e.g. the file is not known to this session).
func serveFile(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	db storage.Storage,
	fileStore files.Storage,
	log *zap.Logger,
	sID, fID string,
) bool {
	// the file must be referenced by one of the session's captured requests (enforces session isolation)
	meta, found := findFileMeta(ctx, db, sID, fID)
	if !found {
		return false
	}

	rc, err := fileStore.Open(ctx, sID, fID)
	if err != nil {
		// metadata exists but the file is missing on disk (e.g. cleaned up) - treat as not found
		log.Warn("file metadata found but content is missing", zap.String("session", sID), zap.String("file", fID))

		return false
	}

	defer func() { _ = rc.Close() }()

	var contentType = meta.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", sanitizeFileName(meta.Name)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return true
	}

	if _, err = io.Copy(w, rc); err != nil {
		log.Error("failed to stream the file", zap.String("session", sID), zap.String("file", fID), zap.Error(err))
	}

	return true
}

// findFileMeta looks up the metadata of a file (by its UUID) among all requests of the session.
func findFileMeta(ctx context.Context, db storage.Storage, sID, fID string) (storage.RequestFile, bool) {
	all, err := db.GetAllRequests(ctx, sID)
	if err != nil {
		return storage.RequestFile{}, false
	}

	for _, req := range all {
		for _, f := range req.Files {
			if f.UUID == fID {
				return f, true
			}
		}
	}

	return storage.RequestFile{}, false
}

// sanitizeFileName strips characters that could break the Content-Disposition header.
func sanitizeFileName(name string) string {
	name = strings.NewReplacer("\"", "", "\\", "", "\r", "", "\n", "").Replace(name)
	if name == "" {
		return "file"
	}

	return name
}

// extractMultipartFiles parses a multipart/form-data body, stores every file part in the file storage, and
// returns the extracted files' metadata together with a sanitized body. The sanitized body preserves the form
// structure (regular fields and file part headers) but replaces file contents with a short placeholder, so the
// file bytes are never stored in the request storage. On any parse error it returns what it managed to extract
// along with the original body unchanged when no files were found.
func extractMultipartFiles(
	ctx context.Context,
	fileStore files.Storage,
	sID string,
	body []byte,
	boundary string,
	log *zap.Logger,
) ([]storage.RequestFile, []byte) {
	var (
		reader    = multipart.NewReader(bytes.NewReader(body), boundary)
		extracted []storage.RequestFile
		buf       bytes.Buffer
		writer    = multipart.NewWriter(&buf)
	)

	// keep the same boundary so the sanitized body stays consistent with the original Content-Type header
	_ = writer.SetBoundary(boundary)

	for {
		part, err := reader.NextPart()
		if err != nil {
			break // io.EOF or a malformed body - stop and keep what we have
		}

		if part.FileName() != "" { // a file part
			var (
				fileUUID    = uuid.New().String()
				contentType = part.Header.Get("Content-Type")
			)

			size, sErr := fileStore.Create(ctx, sID, fileUUID, part)
			if sErr != nil {
				log.Error("failed to store an uploaded file", zap.String("session", sID), zap.Error(sErr))
				_ = part.Close()

				continue
			}

			extracted = append(extracted, storage.RequestFile{
				UUID:        fileUUID,
				Name:        part.FileName(),
				ContentType: contentType,
				Size:        size,
			})

			if ph, wErr := writer.CreateFormFile(part.FormName(), part.FileName()); wErr == nil {
				_, _ = fmt.Fprintf(ph, "[stored file: %s (%d bytes), id: %s]", part.FileName(), size, fileUUID)
			}
		} else { // a regular form field - copy its value through
			if fw, wErr := writer.CreateFormField(part.FormName()); wErr == nil {
				_, _ = io.Copy(fw, part)
			}
		}

		_ = part.Close()
	}

	_ = writer.Close()

	if len(extracted) == 0 {
		return nil, body // nothing was extracted - keep the original body unchanged
	}

	return extracted, buf.Bytes()
}

// TODO: add supporting of format requested by the user (json, html, plain text, etc).
func respondWithError(w http.ResponseWriter, log *zap.Logger, code int, msg string) {
	var s strings.Builder

	s.Grow(1024) //nolint:mnd

	s.WriteString(`<!doctype html>
<html lang="en">
<head>
    <meta charset="utf-8"/>
    <meta http-equiv="X-UA-Compatible" content="IE=edge"/>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
    <title>`)
	s.WriteString(http.StatusText(code))
	s.WriteString(`</title>
    <style>
        html,body {width:100%; height:100%; margin:0; padding:0; background-color: #2b2b2b; color: #efeffa}
        body {display:flex; justify-content:center; align-items:center; font-family:sans-serif}
        .container {text-align:center}
    </style>
</head>
<body>
    <div class="container">
        <h1>WebHook: `)
	s.WriteString(http.StatusText(code))
	s.WriteString(`</h1>
        <h3>`)
	s.WriteString(msg)
	s.WriteString(`</h3>
    </div>
</body>
</html>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(s.Len()))
	w.WriteHeader(code)

	if _, err := w.Write([]byte(s.String())); err != nil {
		log.Error("failed to respond with an error", zap.Error(err), zap.Int("code", code), zap.String("msg", msg))
	}
}

// extractFullUrl returns the full URL from the request.
func extractFullUrl(r *http.Request) string {
	var scheme = "http"
	if r.TLS != nil {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)
}

// we will trust following HTTP headers for the real ip extracting (priority low -> high).
var trustHeaders = [...]string{"X-Forwarded-For", "X-Real-IP", "CF-Connecting-IP"} //nolint:gochecknoglobals

func extractRealIP(r *http.Request) string {
	var ip string

	for _, name := range trustHeaders {
		if value := r.Header.Get(name); value != "" {
			// `X-Forwarded-For` can be `10.0.0.1, 10.0.0.2, 10.0.0.3`
			if strings.Contains(value, ",") {
				parts := strings.Split(value, ",")

				if len(parts) > 0 {
					ip = strings.TrimSpace(parts[0])
				}
			} else {
				ip = strings.TrimSpace(value)
			}
		}
	}

	if net.ParseIP(ip) != nil {
		return ip
	}

	return strings.Split(r.RemoteAddr, ":")[0]
}

func sleep(ctx context.Context, d time.Duration) {
	var timer = time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
