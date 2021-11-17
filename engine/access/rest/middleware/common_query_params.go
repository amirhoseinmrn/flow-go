package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type ctxKeyType string

const expandQueryParam = "expand"
const selectQueryParam = "select"

// commonQueryParamMiddleware generates a Middleware function that extracts the given query parameter from the request
// and adds it to the request context as a key value pair with the key as the query param name.
// e.g. for queryParamName "fields", if the request url contains <some url>?fields=field1,field2,..fieldN,
// the middleware returned by commonQueryParamMiddleware will add the key - "fields" to the request context with value
// ["field", "fields2",..."fieldn"] when it is executed
func commonQueryParamMiddleware(queryParamName string) mux.MiddlewareFunc {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if values, ok := req.URL.Query()[queryParamName]; ok {
				values := strings.Split(values[0], ",")
				// save the query param value in the request context
				contextKey := ctxKeyType(queryParamName)
				req = req.WithContext(context.WithValue(req.Context(), contextKey, values))
			}
			handler.ServeHTTP(w, req)
		})
	}
}

// QueryExpandable middleware extracts out the 'expand' query param field if present in the request
func QueryExpandable() mux.MiddlewareFunc {
	return commonQueryParamMiddleware(expandQueryParam)
}

// QuerySelect middleware extracts out the 'select' query param field if present in the request
func QuerySelect() mux.MiddlewareFunc {
	return commonQueryParamMiddleware(selectQueryParam)
}

func getField(ctx context.Context, key string) ([]string, bool) {
	contextKey := ctxKeyType(key)
	u, ok := ctx.Value(contextKey).([]string)
	return u, ok
}

func GetFieldsToExpand(ctx context.Context) ([]string, bool) {
	return getField(ctx, expandQueryParam)
}

func GetFieldsToSelect(ctx context.Context) ([]string, bool) {
	return getField(ctx, selectQueryParam)
}
