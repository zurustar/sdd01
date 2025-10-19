// Package http provides HTTP handlers and middleware for the scheduler API.
//
// The router exposes the following endpoints:
//   - POST /login: issues a session token. Body: {"email","password"}. Response:
//     {"token","expires_at","principal":{"user_id","is_admin"}} with token also
//     surfaced via the `X-Session-Token` header and a `session_token` cookie.
//   - POST /logout: revokes the current session token extracted from the Authorization
//     header or session cookie. Returns 204 No Content and clears the cookie.
//   - GET /users, POST /users, PUT /users/{id}, DELETE /users/{id}: administrator
//     controlled user management endpoints exchanging the `userDTO` payload defined in
//     user_handler.go.
//   - GET /rooms, POST /rooms, PUT /rooms/{id}, DELETE /rooms/{id}: room catalog
//     endpoints exchanging the `roomDTO` payload defined in room_handler.go. Listing is
//     available to any authenticated principal while mutations require admin privileges.
//   - GET /schedules, POST /schedules, PUT /schedules/{id}, DELETE /schedules/{id}:
//     schedule management endpoints exchanging the `scheduleDTO` payload defined in
//     schedule_handler.go. Schedule responses include conflict warnings and expanded
//     recurrence occurrences.
//
// Request/response DTOs live alongside their respective handlers so tests and
// documentation share the same ground truth.
package http
