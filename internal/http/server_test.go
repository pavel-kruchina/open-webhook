package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"gh.tarampamp.am/webhook-tester/v2/internal/config"
	"gh.tarampamp.am/webhook-tester/v2/internal/files"
	appHttp "gh.tarampamp.am/webhook-tester/v2/internal/http"
	"gh.tarampamp.am/webhook-tester/v2/internal/pubsub"
	"gh.tarampamp.am/webhook-tester/v2/internal/storage"
)

func TestServer_StartHTTP(t *testing.T) {
	t.Parallel()

	var (
		ctx = context.Background()
		log = zap.NewNop()
		srv = appHttp.NewServer(ctx, log)
		db  = storage.NewInMemory(time.Minute, 8)
	)

	t.Cleanup(func() { require.NoError(t, db.Close()) })

	const webhookResponse = "CAPTURED !!! OLOLO"

	sID, err := db.NewSession(ctx, storage.Session{
		Code:         http.StatusExpectationFailed,
		ResponseBody: []byte(webhookResponse),
		Headers:      []storage.HttpHeader{{Name: "Content-Type", Value: "text/someShit"}},
	})
	require.NoError(t, err)

	rID, err := db.NewRequest(ctx, sID, storage.Request{})
	require.NoError(t, err)

	srv.Register(
		context.Background(),
		log,
		func(context.Context) error { return nil },
		func(context.Context) (string, error) { return "v1.0.0", nil },
		&config.AppSettings{},
		db,
		files.NewLocal(t.TempDir()),
		pubsub.NewInMemory[pubsub.RequestEvent](),
		false,
	)

	var baseUrl, stop = startServer(t, ctx, srv)

	t.Cleanup(stop)

	t.Run("index", func(t *testing.T) {
		t.Parallel()

		var status, body, headers = sendRequest(t, "GET", baseUrl)

		require.Equal(t, http.StatusOK, status)
		require.Contains(t, string(body), "<html")
		require.Contains(t, headers.Get("Content-Type"), "text/html")
	})

	t.Run("robots.txt", func(t *testing.T) {
		t.Parallel()

		var status, body, headers = sendRequest(t, "GET", baseUrl+"/////robots.txt")

		require.Equal(t, http.StatusOK, status)
		require.Contains(t, string(body), "User-agent")
		require.Contains(t, string(body), "Disallow")
		require.Contains(t, headers.Get("Content-Type"), "text/plain")
	})

	t.Run("SPA 404", func(t *testing.T) {
		t.Parallel()

		var status, body, headers = sendRequest(t, "GET", baseUrl+"/foo-404")

		require.Equal(t, http.StatusOK, status)
		require.Contains(t, string(body), "<html")
		require.Contains(t, headers.Get("Content-Type"), "text/html")
	})

	t.Run("API 404", func(t *testing.T) {
		t.Parallel()

		var status, body, headers = sendRequest(t, "GET", baseUrl+"/////api/foo-404")

		require.Equal(t, http.StatusNotFound, status)
		require.Contains(t, string(body), "not found")
		require.Contains(t, headers.Get("Content-Type"), "application/json")
	})

	t.Run("ready handler (outside /api prefix)", func(t *testing.T) {
		t.Parallel()

		var status, body, headers = sendRequest(t, "GET", baseUrl+"/ready")

		require.Equal(t, http.StatusOK, status)
		require.Contains(t, string(body), "OK")
		require.Contains(t, headers.Get("Content-Type"), "text/plain")
	})

	t.Run("api handler", func(t *testing.T) {
		t.Parallel()

		var status, body, headers = sendRequest(t, "GET", baseUrl+"/api/settings")

		require.Equal(t, http.StatusOK, status)
		require.Contains(t, string(body), "{")
		require.Contains(t, headers.Get("Content-Type"), "application/json")
	})

	t.Run("openapi spec (machine-readable)", func(t *testing.T) {
		t.Parallel()

		var status, body, headers = sendRequest(t, "GET", baseUrl+"/openapi.json")

		require.Equal(t, http.StatusOK, status)
		require.Contains(t, headers.Get("Content-Type"), "application/json")
		require.Contains(t, string(body), `"openapi"`) // the OpenAPI version field
		require.Contains(t, string(body), "/api/session")
	})

	t.Run("swagger ui docs page", func(t *testing.T) {
		t.Parallel()

		var status, body, headers = sendRequest(t, "GET", baseUrl+"/docs")

		require.Equal(t, http.StatusOK, status)
		require.Contains(t, headers.Get("Content-Type"), "text/html")
		require.Contains(t, string(body), "swagger-ui")
		require.Contains(t, string(body), "openapi.json")
	})

	t.Run("webhook capture", func(t *testing.T) {
		t.Parallel()

		var status, body, headers = sendRequest(t, "POST", baseUrl+"/"+sID)

		require.Equal(t, http.StatusExpectationFailed, status)
		require.Contains(t, string(body), webhookResponse)
		require.Contains(t, headers.Get("Content-Type"), "text/someShit")
		require.Equal(t, headers.Get("Access-Control-Allow-Origin"), "*")
		require.Equal(t, headers.Get("Access-Control-Allow-Methods"), "*")
		require.Equal(t, headers.Get("Access-Control-Allow-Headers"), "*")
	})

	t.Run("API routes exists", func(t *testing.T) {
		t.Parallel()

		for i, params := range []struct{ method, url string }{ // order matters
			{http.MethodPost, "/api/session"},
			{http.MethodGet, "/api/session/" + sID},
			{http.MethodGet, "/api/session/" + sID + "/requests"},
			{http.MethodGet, "/api/session/" + sID + "/requests/subscribe"},
			{http.MethodGet, "/api/session/" + sID + "/requests/" + rID},
			{http.MethodGet, "/api/settings"},
			{http.MethodGet, "/api/version"},
			{http.MethodGet, "/api/version/latest"},
			{http.MethodDelete, "/api/session/" + sID + "/requests/" + rID},
			{http.MethodDelete, "/api/session/" + sID + "/requests"},
			{http.MethodDelete, "/api/session/" + sID},
		} {
			t.Run(fmt.Sprintf("(%d) %s %s", i, params.method, params.url), func(t *testing.T) {
				var status, body, headers = sendRequest(t, params.method, baseUrl+params.url)

				require.NotEqual(t, http.StatusNotFound, status)
				require.NotEmpty(t, body)
				require.Contains(t, headers.Get("Content-Type"), "application/json")
			})
		}
	})
}

func TestServer_PublicURLRoot(t *testing.T) {
	t.Parallel()

	var (
		ctx = context.Background()
		log = zap.NewNop()
		srv = appHttp.NewServer(ctx, log)
		db  = storage.NewInMemory(time.Minute, 8)
	)

	t.Cleanup(func() { require.NoError(t, db.Close()) })

	// Configure PublicURLRoot
	publicURLRoot, err := url.Parse("https://example.com")
	require.NoError(t, err)

	srv.Register(
		context.Background(),
		log,
		func(context.Context) error { return nil },
		func(context.Context) (string, error) { return "v1.0.0", nil },
		&config.AppSettings{PublicURLRoot: publicURLRoot},
		db,
		files.NewLocal(t.TempDir()),
		pubsub.NewInMemory[pubsub.RequestEvent](),
		false,
	)

	var baseUrl, stop = startServer(t, ctx, srv)

	t.Cleanup(stop)

	t.Run("api settings includes public_url_root", func(t *testing.T) {
		t.Parallel()

		var status, body, headers = sendRequest(t, "GET", baseUrl+"/api/settings")

		require.Equal(t, http.StatusOK, status)
		require.Contains(t, headers.Get("Content-Type"), "application/json")
		require.Contains(t, string(body), `"public_url_root":"https://example.com"`)
	})
}

func TestServer_MultipartFilesUploadAndDownload(t *testing.T) {
	t.Parallel()

	var (
		ctx       = context.Background()
		log       = zap.NewNop()
		srv       = appHttp.NewServer(ctx, log)
		db        = storage.NewInMemory(time.Minute, 8)
		fileStore = files.NewLocal(t.TempDir())
	)

	t.Cleanup(func() { require.NoError(t, db.Close()) })

	sID, err := db.NewSession(ctx, storage.Session{Code: http.StatusOK})
	require.NoError(t, err)

	srv.Register(
		context.Background(),
		log,
		func(context.Context) error { return nil },
		func(context.Context) (string, error) { return "v1.0.0", nil },
		&config.AppSettings{SessionTTL: time.Minute},
		db,
		fileStore,
		pubsub.NewInMemory[pubsub.RequestEvent](),
		false,
	)

	var baseUrl, stop = startServer(t, ctx, srv)

	t.Cleanup(stop)

	const (
		fileContent = "the quick brown fox\x00\x01\x02 binary payload"
		fileName    = "report.bin"
	)

	// build and send a multipart/form-data request with a regular field and a file
	var (
		buf = new(bytes.Buffer)
		mw  = multipart.NewWriter(buf)
	)

	require.NoError(t, mw.WriteField("comment", "hello"))

	part, pErr := mw.CreateFormFile("attachment", fileName)
	require.NoError(t, pErr)
	_, _ = part.Write([]byte(fileContent))
	require.NoError(t, mw.Close())

	uploadResp, uErr := http.Post(baseUrl+"/"+sID, mw.FormDataContentType(), buf) //nolint:noctx
	require.NoError(t, uErr)
	require.NoError(t, uploadResp.Body.Close())
	require.Equal(t, http.StatusOK, uploadResp.StatusCode)

	// fetch the captured request list and extract the stored file metadata
	listStatus, listBody, _ := sendRequest(t, http.MethodGet, baseUrl+"/api/session/"+sID+"/requests")
	require.Equal(t, http.StatusOK, listStatus)

	var list []struct {
		UUID                 string `json:"uuid"`
		RequestPayloadBase64 string `json:"request_payload_base64"`
		Files                []struct {
			UUID        string `json:"uuid"`
			Name        string `json:"name"`
			ContentType string `json:"content_type"`
			Size        int64  `json:"size"`
		} `json:"files"`
	}

	require.NoError(t, json.Unmarshal(listBody, &list))
	require.Len(t, list, 1)
	require.Len(t, list[0].Files, 1, "the uploaded file must be extracted as metadata")

	var file = list[0].Files[0]
	require.Equal(t, fileName, file.Name)
	require.Equal(t, int64(len(fileContent)), file.Size)
	require.NotEmpty(t, file.UUID)

	t.Run("download the stored file", func(t *testing.T) {
		status, body, headers := sendRequest(t, http.MethodGet, baseUrl+"/"+sID+"/files/"+file.UUID)

		require.Equal(t, http.StatusOK, status)
		require.Equal(t, fileContent, string(body), "downloaded content must match the upload")
		require.Contains(t, headers.Get("Content-Disposition"), fileName)
		require.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
	})

	t.Run("unknown file id is not served (falls through to capture)", func(t *testing.T) {
		status, _, headers := sendRequest(t, http.MethodGet, baseUrl+"/"+sID+"/files/"+uuid.NewString())

		require.Equal(t, http.StatusOK, status) // captured as a normal webhook request
		require.Empty(t, headers.Get("Content-Disposition"), "must not serve a file for an unknown id")
	})
}

// sendRequest is a helper function to send an HTTP request and return its status code, body, and headers.
func sendRequest(t *testing.T, method, url string, headers ...map[string]string) (
	status int,
	body []byte,
	_ http.Header,
) {
	t.Helper()

	req, reqErr := http.NewRequest(method, url, nil)

	require.NoError(t, reqErr)

	if len(headers) > 0 {
		for key, value := range headers[0] {
			req.Header.Add(key, value)
		}
	}

	resp, err := http.DefaultClient.Do(req) //nolint:gosec
	require.NoError(t, err)

	body, _ = io.ReadAll(resp.Body)

	require.NoError(t, resp.Body.Close())

	return resp.StatusCode, body, resp.Header
}

// startServer is a helper function to start an HTTP server and return its base URL.
func startServer(t *testing.T, pCtx context.Context, srv interface {
	StartHTTP(ctx context.Context, ln net.Listener) error
}) (string /* baseurl */, func() /* stop */) {
	t.Helper()

	var (
		port     = getFreeTcpPort(t)
		hostPort = net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port)) //nolint:govet
	)

	// open HTTP port
	ln, lnErr := net.Listen("tcp", hostPort)
	require.NoError(t, lnErr)

	var ctx, cancel = context.WithCancel(pCtx)

	go func() {
		err := srv.StartHTTP(ctx, ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			require.NoError(t, err)
		}
	}()

	// wait until the server starts
	for {
		if conn, err := net.DialTimeout("tcp", hostPort, time.Second); err == nil {
			require.NoError(t, conn.Close())

			break
		}

		<-time.After(5 * time.Millisecond)
	}

	return fmt.Sprintf("http://%s", hostPort), cancel
}

// getFreeTcpPort is a helper function to get a free TCP port number.
func getFreeTcpPort(t *testing.T) uint16 {
	t.Helper()

	l, lErr := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, lErr)

	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())

	// make sure port is closed
	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			break
		}

		require.NoError(t, conn.Close())
		<-time.After(5 * time.Millisecond)
	}

	return uint16(port) //nolint:gosec
}
