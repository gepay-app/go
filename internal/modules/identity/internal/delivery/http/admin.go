package http

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
	"uuid"

	"app/internal/modules/identity/api"
	"app/internal/modules/identity/internal/service"
	"app/internal/platform/apperr"
	"app/internal/platform/server/middleware"
	"app/internal/platform/utils"
)

// actor me-resolve caller yang sedang login (dipakai untuk cek role admin).
func (h *handler) actor(c *echo.Context) (api.User, error) {
	id, ok := middleware.Identity(c.Request().Context())
	if !ok {
		return api.User{}, apperr.Unauthorized("unauthorized", "unauthenticated")
	}
	return h.svc.Resolve(c.Request().Context(), id.UID)
}

func (h *handler) userIDParam(c *echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.UUID{}, apperr.Invalid("invalid_id", "invalid user id")
	}
	return id, nil
}

func (h *handler) createUser(c *echo.Context) error {
	actor, err := h.actor(c)
	if err != nil {
		return err
	}

	var req createUserRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.validate(); err != nil {
		return err
	}

	u, err := h.svc.CreateUser(c.Request().Context(), actor, service.CreateUserInput{
		Email: req.Email,
		Name:  req.Name,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{"data": u})
}

func (h *handler) listUsers(c *echo.Context) error {
	actor, err := h.actor(c)
	if err != nil {
		return err
	}

	cursorID := uuid.Max() // halaman pertama: lebih besar dari id mana pun
	if raw := c.QueryParam("cursor"); raw != "" {
		idStr, err := utils.DecodeCursor(raw)
		if err != nil {
			return apperr.Validation(apperr.FieldError{Field: "cursor", Message: "invalid cursor"})
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			return apperr.Validation(apperr.FieldError{Field: "cursor", Message: "invalid cursor"})
		}
		cursorID = id
	}

	limit := utils.ParseLimit(c.QueryParam("limit"), 20, 100)
	users, hasMore, err := h.svc.ListUsers(c.Request().Context(), actor, cursorID, limit)
	if err != nil {
		return err
	}

	next := ""
	if hasMore && len(users) > 0 {
		next = utils.EncodeCursor(users[len(users)-1].ID.String())
	}
	return c.JSON(http.StatusOK, map[string]any{"data": users, "next_cursor": next})
}

func (h *handler) deleteUser(c *echo.Context) error {
	actor, err := h.actor(c)
	if err != nil {
		return err
	}
	id, err := h.userIDParam(c)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteUser(c.Request().Context(), actor, id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *handler) updateStatus(c *echo.Context) error {
	actor, err := h.actor(c)
	if err != nil {
		return err
	}
	id, err := h.userIDParam(c)
	if err != nil {
		return err
	}

	var req updateStatusRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.validate(); err != nil {
		return err
	}

	if err := h.svc.UpdateStatus(c.Request().Context(), actor, id, req.Status); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *handler) grantRole(c *echo.Context) error {
	actor, err := h.actor(c)
	if err != nil {
		return err
	}
	id, err := h.userIDParam(c)
	if err != nil {
		return err
	}

	var req grantRoleRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.validate(); err != nil {
		return err
	}

	grantedBy := pgtype.UUID{Bytes: actor.ID, Valid: true}
	if err := h.svc.GrantRole(c.Request().Context(), actor, id, req.Role, grantedBy); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *handler) revokeRole(c *echo.Context) error {
	actor, err := h.actor(c)
	if err != nil {
		return err
	}
	id, err := h.userIDParam(c)
	if err != nil {
		return err
	}

	// Role dari path param: divalidasi di transport, sama seperti body request.
	role := api.Role(c.Param("role"))
	if !role.Valid() {
		return apperr.Validation(apperr.FieldError{Field: "role", Message: "invalid role"})
	}

	if err := h.svc.RevokeRole(c.Request().Context(), actor, id, role); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
