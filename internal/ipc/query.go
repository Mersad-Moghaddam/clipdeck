package ipc

import (
	"encoding/json"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

func (s *Server) handleQuery(params json.RawMessage) (any, *RPCError) {
	var opts store.QueryOptions
	if len(params) > 0 {
		if err := json.Unmarshal(params, &opts); err != nil {
			return nil, &RPCError{Code: -32602, Message: "Invalid query parameters"}
		}
	}
	if opts.Limit < 0 || opts.Limit > 200 || opts.Offset < 0 {
		return nil, &RPCError{Code: -32602, Message: "limit must be 0..200 and offset nonnegative"}
	}
	valid := map[string]bool{"text": true, "code": true, "json": true, "link": true, "color": true, "image": true, "email": true, "phone": true, "number": true, "file": true, "path": true}
	for _, kind := range opts.Types {
		if !valid[kind] {
			return nil, &RPCError{Code: -32602, Message: "Unknown query type"}
		}
	}
	page, err := s.store.QueryItems(opts)
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: "Query failed"}
	}
	page.Items = slimForList(page.Items)
	for i := range page.Items {
		item := &page.Items[i]
		if item.Sensitive {
			item.Content = item.MaskedContent
			item.Preview = item.MaskedContent
			item.RichPreview = ""
			item.BlobPath = ""
			item.ThumbnailPath = ""
		}
	}
	return page, nil
}
