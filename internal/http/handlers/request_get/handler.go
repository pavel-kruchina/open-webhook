package request_get

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/google/uuid"

	"gh.tarampamp.am/webhook-tester/v2/internal/http/openapi"
	"gh.tarampamp.am/webhook-tester/v2/internal/storage"
)

type (
	sID = openapi.SessionUUIDInPath
	rID = openapi.RequestUUIDInPath

	Handler struct{ db storage.Storage }
)

func New(db storage.Storage) *Handler { return &Handler{db: db} }

func (h *Handler) Handle(ctx context.Context, sID sID, rID rID) (*openapi.CapturedRequestsResponse, error) {
	r, rErr := h.db.GetRequest(ctx, sID.String(), rID.String())
	if rErr != nil {
		return nil, rErr
	}

	var rHeaders = make([]openapi.HttpHeader, len(r.Headers))
	for i, header := range r.Headers {
		rHeaders[i].Name, rHeaders[i].Value = header.Name, header.Value
	}

	return &openapi.CapturedRequestsResponse{
		CapturedAtUnixMilli:  r.CreatedAtUnixMilli,
		ClientAddress:        r.ClientAddr,
		Headers:              rHeaders,
		Method:               strings.ToUpper(r.Method),
		RequestPayloadBase64: base64.StdEncoding.EncodeToString(r.Body),
		Url:                  r.URL,
		Uuid:                 rID,
		Files:                convertFiles(r.Files),
	}, nil
}

// convertFiles maps storage file metadata to the OpenAPI representation.
func convertFiles(in []storage.RequestFile) []openapi.RequestFile {
	var out = make([]openapi.RequestFile, 0, len(in))
	for _, f := range in {
		fUUID, err := uuid.Parse(f.UUID)
		if err != nil {
			continue
		}

		out = append(out, openapi.RequestFile{
			Uuid:        fUUID,
			Name:        f.Name,
			ContentType: f.ContentType,
			Size:        f.Size,
		})
	}

	return out
}
