package token

import (
	"errors"
	"github.com/golang-jwt/jwt/v4"
	"net/http"
	"time"
	"web/csgo"
)

const JWTToken = "csgo_token"

type JwtHandler struct {
	//算法
	Alg string
	//登录认证的方法，认证后在执行生成jwt
	Authenticator func(ctx *csgo.Context) (map[string]any, error)
	//过期时间 秒
	TimeOut time.Duration
	//刷新token的过期时间
	RefreshTimeOut time.Duration
	//时间函数 从此时开始计算过期
	TimeFuc func() time.Time
	//私钥
	PrivateKey string
	//key
	Key []byte
	//刷新key
	RefreshKey string
	//save cookie
	SendCookie     bool
	CookieName     string
	CookieMaxAge   int64
	CookieDomain   string
	SecureCookie   bool
	CookieHTTPOnly bool
	Head           string
	AuthHandler    func(ctx *csgo.Context, err error)
}

type JwtResponse struct {
	Token        string
	RefreshToken string
}

//  login(user) -->id --> jwt --->save cookie

// LoginHandler  登录请求到来时，进行认证，然后返回一个token
func (j *JwtHandler) LoginHandler(ctx *csgo.Context) (*JwtResponse, error) {
	data, err := j.Authenticator(ctx)
	if err != nil {
		return nil, err
	}
	if j.Alg == "" {
		j.Alg = "HS256"
	}
	signingMethod := jwt.GetSigningMethod(j.Alg)
	token := jwt.New(signingMethod)
	claims := token.Claims.(jwt.MapClaims)
	if data != nil {
		for key, value := range data {
			claims[key] = value
		}
	}
	if j.TimeFuc == nil {
		j.TimeFuc = func() time.Time {
			return time.Now()
		}
	}
	expire := j.TimeFuc().Add(j.TimeOut)
	claims["exp"] = expire.Unix()
	claims["iat"] = j.TimeFuc().Unix()
	var tokenString string
	var errToken error
	if j.usingPublicKeyAlgo() {
		tokenString, errToken = token.SignedString(j.PrivateKey)
	} else {
		tokenString, errToken = token.SignedString(j.Key)
	}
	if errToken != nil {
		return nil, err
	}
	if j.SendCookie {
		if j.CookieName == "" {
			j.CookieName = JWTToken
		}
		if j.CookieMaxAge == 0 {
			j.CookieMaxAge = int64(int(expire.Unix() - j.TimeFuc().Unix()))
		}
		maxAge := j.CookieMaxAge
		ctx.SetCookie(j.CookieName, tokenString, int(maxAge), "/", j.CookieDomain, j.SecureCookie, j.CookieHTTPOnly)
	}
	jr := &JwtResponse{}

	refreshToken, err := j.refreshToken(token)
	if err != nil {
		return nil, err
	}
	jr.Token = tokenString
	jr.RefreshToken = refreshToken
	return jr, nil
}

func (j *JwtHandler) usingPublicKeyAlgo() bool {
	switch j.Alg {
	case "RS256", "RS512", "RS384":
		return true
	}
	return false
}

//设置第二时间过期的token
func (j *JwtHandler) refreshToken(token *jwt.Token) (string, error) {
	claims := token.Claims.(jwt.MapClaims)

	claims["exp"] = j.TimeFuc().Add(j.RefreshTimeOut)
	var tokenString string
	var tokenErr error
	if j.usingPublicKeyAlgo() {
		tokenString, tokenErr = token.SignedString(j.PrivateKey)
	} else {
		//如果不是那么使用公钥进行加密
		tokenString, tokenErr = token.SignedString(j.Key)
	}
	if tokenErr != nil {
		return "", tokenErr
	}
	return tokenString, nil
}

// LogoutHandler 登出
func (j *JwtHandler) LogoutHandler(ctx *csgo.Context) error {
	//清除cookie即可
	if j.SendCookie {
		if j.CookieName == "" {
			j.CookieName = JWTToken
		}
		ctx.SetCookie(j.CookieName, "", -1, "/", j.CookieDomain, j.SecureCookie, j.CookieHTTPOnly)
		return nil
	}
	return nil
}

// RefreshHandler 刷新token
func (j *JwtHandler) RefreshHandler(ctx *csgo.Context) (*JwtResponse, error) {
	var token string
	//检测refresh token是否过期
	storageToken, exists := ctx.Get(j.RefreshKey)
	if exists {
		token = storageToken.(string)
	}
	if token == "" {
		return nil, errors.New("token not exist")
	}
	t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if j.usingPublicKeyAlgo() {
			return j.PrivateKey, nil
		}
		return j.Key, nil
	})
	if err != nil {
		return nil, err
	}
	claims := t.Claims.(jwt.MapClaims)
	//未过期的情况下 重新生成token和refreshToken
	if j.TimeFuc == nil {
		j.TimeFuc = func() time.Time {
			return time.Now()
		}
	}
	expire := j.TimeFuc().Add(j.RefreshTimeOut)
	claims["exp"] = expire.Unix()
	claims["iat"] = j.TimeFuc().Unix()

	var tokenString string
	var errToken error
	if j.usingPublicKeyAlgo() {
		tokenString, errToken = t.SignedString(j.PrivateKey)
	} else {
		tokenString, errToken = t.SignedString(j.Key)
	}
	if errToken != nil {
		return nil, errToken
	}
	if j.SendCookie {
		if j.CookieName == "" {
			j.CookieName = JWTToken
		}
		if j.CookieMaxAge == 0 {
			j.CookieMaxAge = int64(int(expire.Unix() - j.TimeFuc().Unix()))
		}
		maxAge := j.CookieMaxAge
		ctx.SetCookie(j.CookieName, tokenString, int(maxAge), "/", j.CookieDomain, j.SecureCookie, j.CookieHTTPOnly)
	}
	jr := &JwtResponse{}
	refreshToken, err := j.refreshToken(t)
	if err != nil {
		return nil, err
	}
	jr.Token = tokenString
	jr.RefreshToken = refreshToken
	return jr, nil
}

// AuthInterceptor jwt登录中间件
//检测head中的token是否合法
func (j *JwtHandler) AuthInterceptor(next csgo.HandleFunc) csgo.HandleFunc {
	return func(ctx *csgo.Context) {
		if j.Head == "" {
			j.Head = "Authorization"
		}
		token := ctx.R.Header.Get(j.Head)
		if token == "" {
			if j.SendCookie {
				cookie, err := ctx.R.Cookie(j.CookieName)
				if err != nil {
					j.AuthErrorHandler(ctx, err)
					return
				}
				token = cookie.String()
			}
		}
		if token == "" {
			j.AuthErrorHandler(ctx, errors.New("token is nil"))
			return
		}
		t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			if j.usingPublicKeyAlgo() {
				return j.PrivateKey, nil
			}
			return j.Key, nil
		})
		if err != nil {
			j.AuthErrorHandler(ctx, err)
			return
		}
		ctx.Set("claims", t.Claims.(jwt.MapClaims))
		next(ctx)
	}
}

// AuthErrorHandler 处理token认证错误
func (j *JwtHandler) AuthErrorHandler(ctx *csgo.Context, err error) {
	if j.AuthHandler == nil {
		ctx.W.WriteHeader(http.StatusUnauthorized)
	} else {
		j.AuthHandler(ctx, err)
	}
}
