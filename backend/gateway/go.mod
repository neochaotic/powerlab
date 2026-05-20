module github.com/neochaotic/powerlab/backend/gateway

go 1.25.7

require (
	github.com/digitorus/pkcs7 v0.0.0-20250730155240-ffadbf3f398c
	github.com/google/uuid v1.6.0
	github.com/grandcat/zeroconf v1.0.0
	github.com/labstack/echo/v4 v4.15.2
	github.com/neochaotic/powerlab/backend/common v0.4.8-alpha9
	github.com/neochaotic/powerlab/backend/pkg v0.0.0-00010101000000-000000000000
	github.com/spf13/viper v1.21.0
	go.uber.org/fx v1.24.0
	gotest.tools v2.2.0+incompatible
)

require (
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/labstack/echo-jwt/v4 v4.4.0 // indirect
	github.com/labstack/gommon v0.5.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/miekg/dns v1.1.27 // indirect
	github.com/orca-zhang/ecache v1.1.3 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/samber/lo v1.53.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/time v0.15.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

require (
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gotest.tools/v3 v3.5.2
)

replace github.com/neochaotic/powerlab/backend/common => ../common

replace github.com/neochaotic/powerlab/backend/pkg => ../pkg
