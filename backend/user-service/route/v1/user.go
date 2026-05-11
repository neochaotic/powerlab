package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/time/rate"

	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/neochaotic/powerlab/backend/user-service/model"
	"github.com/neochaotic/powerlab/backend/user-service/model/system_model"
	"github.com/neochaotic/powerlab/backend/user-service/pkg/config"
	"github.com/neochaotic/powerlab/backend/user-service/pkg/utils/encryption"
	"github.com/neochaotic/powerlab/backend/user-service/pkg/utils/file"
	"github.com/neochaotic/powerlab/backend/user-service/service"
	model2 "github.com/neochaotic/powerlab/backend/user-service/service/model"
)

// @Summary register user
// @Router /user/register/ [post]
func PostUserRegister(ctx echo.Context) error {
	json := make(map[string]string)
	ctx.Bind(&json)

	username := json["username"]
	pwd := json["password"]
	key := json["key"]
	if _, ok := service.UserRegisterHash[key]; !ok {
		return ctx.JSON(common_err.CLIENT_ERROR,
			model.Result{Success: common_err.KEY_NOT_EXIST, Message: common_err.GetMsg(common_err.KEY_NOT_EXIST)})
	}

	if len(username) == 0 || len(pwd) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR,
			model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}
	if len(pwd) < 6 {
		return ctx.JSON(common_err.CLIENT_ERROR,
			model.Result{Success: common_err.PWD_IS_TOO_SIMPLE, Message: common_err.GetMsg(common_err.PWD_IS_TOO_SIMPLE)})
	}
	oldUser := service.MyService.User().GetUserInfoByUserName(username)
	if oldUser.Id > 0 {
		return ctx.JSON(common_err.CLIENT_ERROR,
			model.Result{Success: common_err.USER_EXIST, Message: common_err.GetMsg(common_err.USER_EXIST)})
	}

	user := model2.UserDBModel{}
	user.Username = username
	hashedPassword, err := encryption.HashPassword(pwd)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	user.Password = hashedPassword
	user.Role = "admin"

	user = service.MyService.User().CreateUser(user)
	if user.Id == 0 {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: common_err.GetMsg(common_err.SERVICE_ERROR)})
	}
	file.MkDir(config.AppInfo.UserDataPath + "/" + strconv.Itoa(user.Id))
	delete(service.UserRegisterHash, key)
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

var limiter = rate.NewLimiter(rate.Every(time.Minute), 5)

// @Summary login
// @Produce  application/json
// @Accept application/json
// @Tags user
// @Param user_name query string true "User name"
// @Param pwd  query string true "password"
// @Success 200 {string} string "ok"
// @Router /user/login [post]
func PostUserLogin(ctx echo.Context) error {
	if !limiter.Allow() {
		return ctx.JSON(common_err.TOO_MANY_REQUEST,
			model.Result{
				Success: common_err.TOO_MANY_LOGIN_REQUESTS,
				Message: common_err.GetMsg(common_err.TOO_MANY_LOGIN_REQUESTS),
			})
	}

	json := make(map[string]string)
	ctx.Bind(&json)

	username := json["username"]

	password := json["password"]
	// check params is empty
	if len(username) == 0 || len(password) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR,
			model.Result{
				Success: common_err.CLIENT_ERROR,
				Message: common_err.GetMsg(common_err.INVALID_PARAMS),
			})
	}
	// 1. Try OS authentication first.
	//
	// The OS service distinguishes three return states:
	//   (true,  nil) → password accepted by the OS
	//   (false, nil) → password explicitly rejected by the OS — i.e.
	//                  the user typed the wrong password
	//   (false, err) → OS auth could not run on this host (no PAM,
	//                  dscl unavailable, etc.) — caller must decide
	//                  whether to fall back to bcrypt or surface the
	//                  configuration error.
	//
	// Treating (false, nil) as "try bcrypt next" would let an attacker
	// with a wrong PAM password silently slide into the bcrypt code
	// path, which is correct for users who set up SetupWizard but is
	// confusing UX (the rejection message becomes "OS auth unavailable"
	// even though OS auth worked perfectly fine). Splitting the two
	// states makes the response messages match what actually happened.
	success, osErr := service.MyService.OS().Authenticate(username, password)
	var user model2.UserDBModel

	switch {
	case success:
		// OS Auth Success — sync with the local DB so we have a
		// stable user.Id to mint a JWT against.
		user = service.MyService.User().GetUserAllInfoByName(username)
		if user.Id == 0 {
			// First sign-in for this OS user → mirror into the DB.
			osUser, _ := service.MyService.OS().GetOSUser(username)
			user = model2.UserDBModel{
				Username: username,
				Password: "", // No bcrypt; OS owns the credential.
				Role:     "admin",
				Source:   "os",
				UID:      osUser.Uid,
				Nickname: username,
			}
			user = service.MyService.User().CreateUser(user)
		} else if user.Source == "" {
			// Pre-existing DB row with no source — promote to "os"
			// so future audits know this account flows through PAM.
			user.Source = "os"
			service.MyService.User().UpdateUser(user)
		}

	case !success && osErr == nil:
		// OS auth ran cleanly and rejected the credential. Surface a
		// generic "invalid" error — never the bcrypt-fallback path,
		// which would leak whether or not a SetupWizard password
		// exists for this user.
		return ctx.JSON(common_err.CLIENT_ERROR,
			model.Result{Success: common_err.USER_NOT_EXIST_OR_PWD_INVALID, Message: common_err.GetMsg(common_err.USER_NOT_EXIST_OR_PWD_INVALID)})

	default:
		// OS auth could not run (osErr != nil) → try the bcrypt
		// fallback so a user who registered through SetupWizard on a
		// host without PAM (older Linux build, macOS dev) can still
		// sign in.
		user = service.MyService.User().GetUserAllInfoByName(username)
		if user.Id == 0 {
			return ctx.JSON(common_err.CLIENT_ERROR,
				model.Result{Success: common_err.USER_NOT_EXIST, Message: common_err.GetMsg(common_err.USER_NOT_EXIST)})
		}
		if user.Password == "" {
			// OS-only user with no bcrypt fallback — and OS path is
			// down. Tell the caller why explicitly so they can fix
			// the underlying config issue.
			return ctx.JSON(common_err.CLIENT_ERROR,
				model.Result{Success: common_err.USER_NOT_EXIST_OR_PWD_INVALID, Message: "OS authentication is unavailable and no fallback password is set for this user"})
		}
		if !encryption.CheckPasswordHash(password, user.Password) {
			return ctx.JSON(common_err.CLIENT_ERROR,
				model.Result{Success: common_err.USER_NOT_EXIST_OR_PWD_INVALID, Message: common_err.GetMsg(common_err.USER_NOT_EXIST_OR_PWD_INVALID)})
		}
	}

	privateKey, _ := service.MyService.User().GetKeyPair()

	token := system_model.VerifyInformation{}

	accessToken, err := jwt.GetAccessToken(user.Username, privateKey, user.Id)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	token.AccessToken = accessToken

	refreshToken, err := jwt.GetRefreshToken(user.Username, privateKey, user.Id)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	token.RefreshToken = refreshToken

	token.ExpiresAt = time.Now().Add(3 * time.Hour * time.Duration(1)).Unix()
	data := make(map[string]interface{}, 2)
	user.Password = ""
	data["token"] = token

	// TODO:1 Database fields cannot be external
	data["user"] = user

	return ctx.JSON(common_err.SUCCESS,
		model.Result{
			Success: common_err.SUCCESS,
			Message: common_err.GetMsg(common_err.SUCCESS),
			Data:    data,
		})
}

// @Summary edit user head
// @Produce  application/json
// @Accept multipart/form-data
// @Tags user
// @Param file formData file true "用户头像"
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /users/avatar [put]
func PutUserInfo(ctx echo.Context) error {
	id := ctx.Request().Header.Get("user_id")
	json := model2.UserDBModel{}
	ctx.Bind(&json)
	user := service.MyService.User().GetUserInfoById(id)
	if user.Id == 0 {
		return ctx.JSON(common_err.SERVICE_ERROR,
			model.Result{Success: common_err.USER_NOT_EXIST_OR_PWD_INVALID, Message: common_err.GetMsg(common_err.USER_NOT_EXIST_OR_PWD_INVALID)})
	}
	if len(json.Username) > 0 {
		u := service.MyService.User().GetUserInfoByUserName(json.Username)
		if u.Id > 0 {
			return ctx.JSON(common_err.CLIENT_ERROR,
				model.Result{Success: common_err.USER_EXIST, Message: common_err.GetMsg(common_err.USER_EXIST)})
		}
	}

	if len(json.Email) == 0 {
		json.Email = user.Email
	}
	if len(json.Avatar) == 0 {
		json.Avatar = user.Avatar
	}
	if len(json.Role) == 0 {
		json.Role = user.Role
	}
	if len(json.Description) == 0 {
		json.Description = user.Description
	}
	if len(json.Nickname) == 0 {
		json.Nickname = user.Nickname
	}
	service.MyService.User().UpdateUser(json)
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: json})
}

// @Summary edit user password
// @Produce  application/json
// @Accept application/json
// @Tags user
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /user/password/:id [put]
func PutUserPassword(ctx echo.Context) error {
	id := ctx.Request().Header.Get("user_id")
	json := make(map[string]string)
	ctx.Bind(&json)
	oldPwd := json["old_password"]
	pwd := json["password"]
	if len(oldPwd) == 0 || len(pwd) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}
	user := service.MyService.User().GetUserAllInfoById(id)
	if user.Id == 0 {
		return ctx.JSON(common_err.SERVICE_ERROR,
			model.Result{Success: common_err.USER_NOT_EXIST, Message: common_err.GetMsg(common_err.USER_NOT_EXIST)})
	}
	if !encryption.CheckPasswordHash(oldPwd, user.Password) {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.PWD_INVALID_OLD, Message: common_err.GetMsg(common_err.PWD_INVALID_OLD)})
	}
	hashedPassword, err := encryption.HashPassword(pwd)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	user.Password = hashedPassword
	service.MyService.User().UpdateUserPassword(user)
	user.Password = ""
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: user})
}

// @Summary edit user nick
// @Produce  application/json
// @Accept application/json
// @Tags user
// @Param nick_name query string false "nick name"
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /user/nick [put]
func GetUserInfo(ctx echo.Context) error {
	id := ctx.Request().Header.Get("user_id")
	user := service.MyService.User().GetUserInfoById(id)

	return ctx.JSON(common_err.SUCCESS,
		model.Result{
			Success: common_err.SUCCESS,
			Message: common_err.GetMsg(common_err.SUCCESS),
			Data:    user,
		})
}

// GetUserInfoByUsername returns the public profile (id, nickname,
// avatar, role) for the user with the given `:username` path
// param. Returns INVALID_PARAMS when username is empty.
//
// Route: GET /v1/user/info/:username
func GetUserStatus(ctx echo.Context) error {
	data := make(map[string]interface{}, 2)

	if service.MyService.User().GetUserCount() > 0 {
		data["initialized"] = true
		data["key"] = ""
	} else {
		key := uuid.NewV4().String()
		service.UserRegisterHash[key] = key
		data["key"] = key
		data["initialized"] = false
	}
	gpus, err := external.NvidiaGPUInfoList()
	if err != nil {
		_log.Error(ctx.Request().Context(), "NvidiaGPUInfoList error", err)
	}
	data["gpus"] = len(gpus)
	return ctx.JSON(common_err.SUCCESS,
		model.Result{
			Success: common_err.SUCCESS,
			Message: common_err.GetMsg(common_err.SUCCESS),
			Data:    data,
		})
}
