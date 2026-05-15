module github.com/neochaotic/powerlab/backend/message-bus

go 1.25.7

require (
	github.com/CorrectRoadH/go-socket.io v1.8.0-rc.5
	github.com/glebarez/sqlite v1.7.0
	github.com/gobwas/ws v1.1.0
	github.com/json-iterator/go v1.1.12
	github.com/neochaotic/powerlab/backend/common v0.4.4-alpha2
	github.com/neochaotic/powerlab/backend/pkg v0.0.0-00010101000000-000000000000
	github.com/samber/lo v1.46.0
	go.uber.org/goleak v1.2.1
	gorm.io/gorm v1.24.6
	gotest.tools v2.2.0+incompatible
)

replace github.com/neochaotic/powerlab/backend/pkg => ../pkg

replace github.com/neochaotic/powerlab/backend/common => ../common

require (
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/dsnet/compress v0.0.2-0.20210315054119-f66993602bf5 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/glebarez/go-sqlite v1.21.0 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.0.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gomodule/redigo v2.0.0+incompatible // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/klauspost/pgzip v1.2.5 // indirect
	github.com/labstack/echo-jwt/v4 v4.2.0 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/mholt/archiver/v3 v3.5.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/orca-zhang/ecache v1.1.3 // indirect
	github.com/perimeterx/marshmallow v1.1.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pressly/goose/v3 v3.27.1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/tidwall/gjson v1.17.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	modernc.org/libc v1.72.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.50.1 // indirect
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/deepmap/oapi-codegen v1.12.4
	github.com/getkin/kin-openapi v0.115.0
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/google/uuid v1.6.0
	github.com/invopop/yaml v0.2.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/labstack/echo/v4 v4.12.0
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.21 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	gopkg.in/ini.v1 v1.67.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
