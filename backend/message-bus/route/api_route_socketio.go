package route

import "github.com/labstack/echo/v4"

// SubscribeSIO mounts the socketio server on the WS-upgrade path.
//
// Route: GET /v1/socket.io (Upgrade: websocket)
func (r *APIRoute) SubscribeSIO(ctx echo.Context) error {
	server := r.services.SocketIOService.Server()
	server.ServeHTTP(ctx.Response(), ctx.Request())
	return nil
}

// SubscribeSIO2 is the trailing-slash duplicate of SubscribeSIO.
// Both /socket.io and /socket.io/ have to be wired explicitly because
// echo's router treats them as separate routes.
//
// Route: GET /v1/socket.io/
func (r *APIRoute) SubscribeSIO2(ctx echo.Context) error {
	return r.SubscribeSIO(ctx)
}

// PollSIO mounts the socketio server on the long-polling fallback
// path used when WebSocket upgrade is unavailable.
//
// Route: GET /v1/socket.io
func (r *APIRoute) PollSIO(ctx echo.Context) error {
	server := r.services.SocketIOService.Server()
	server.ServeHTTP(ctx.Response(), ctx.Request())
	return nil
}

// PollSIO2 is the trailing-slash duplicate of PollSIO.
//
// Route: GET /v1/socket.io/
func (r *APIRoute) PollSIO2(ctx echo.Context) error {
	return r.PollSIO(ctx)
}
