// Package apijson provides small HTTP helpers for the task JSON API: baseline security
// headers and JSON error responses (including request_id when present on context).
//
// Handlers pass an optional callPath function for debug http.io logs; middleware outside
// pkgs/tasks/handler can pass nil.
package apijson
