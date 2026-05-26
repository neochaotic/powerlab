module github.com/neochaotic/powerlab/backend/local-storage

go 1.25.7

require (
	bazil.org/fuse v0.0.0-20230120002735-62a210ff1fd5
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/deckarep/golang-set/v2 v2.3.0
	github.com/deepmap/oapi-codegen v1.12.4
	github.com/getkin/kin-openapi v0.138.0
	github.com/glebarez/sqlite v1.8.0
	github.com/json-iterator/go v1.1.12
	github.com/labstack/echo-jwt/v4 v4.4.0
	github.com/labstack/echo/v4 v4.13.4
	github.com/maruel/natural v1.1.0
	github.com/moby/sys/mountinfo v0.7.2
	github.com/neochaotic/powerlab/backend/common v0.4.9-alpha6
	github.com/neochaotic/powerlab/backend/pkg v0.0.0-20260509194312-a9f601c6bd80
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pilebones/go-udev v0.9.0
	github.com/rclone/rclone v1.73.5
	github.com/robfig/cron/v3 v3.0.1
	github.com/samber/lo v1.53.0
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/shirou/gopsutil/v3 v3.23.2
	github.com/tidwall/gjson v1.19.0
	gopkg.in/ini.v1 v1.67.0
	gorm.io/gorm v1.25.0
	gotest.tools/v3 v3.5.2
)

require (
	cloud.google.com/go/auth v0.17.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.20.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.13.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.3 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/storage/azfile v1.5.3 // indirect
	github.com/Azure/go-ntlmssp v0.0.2-0.20251110135918-10b7b7e7cd26 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.6.0 // indirect
	github.com/FilenCloudDienste/filen-sdk-go v0.0.38 // indirect
	github.com/Files-com/files-sdk-go/v3 v3.2.264 // indirect
	github.com/IBM/go-sdk-core/v5 v5.18.5 // indirect
	github.com/Max-Sum/base32768 v0.0.0-20230304063302-18e6ce5945fd // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/bcrypt v0.0.0-20211005172633-e235017c1baf // indirect
	github.com/ProtonMail/gluon v0.17.1-0.20230724134000-308be39be96e // indirect
	github.com/ProtonMail/go-crypto v1.3.0 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/go-srp v0.0.7 // indirect
	github.com/ProtonMail/gopenpgp/v2 v2.9.0 // indirect
	github.com/PuerkitoBio/goquery v1.10.3 // indirect
	github.com/STARRY-S/zip v0.2.3 // indirect
	github.com/a1ex3/zstd-seekable-format-go/pkg v0.10.0 // indirect
	github.com/a8m/tree v0.0.0-20240104212747-2c8764a5f17e // indirect
	github.com/aalpar/deheap v0.0.0-20210914013432-0cc84d79dec3 // indirect
	github.com/abbot/go-http-auth v0.4.0 // indirect
	github.com/anacrolix/dms v1.7.2 // indirect
	github.com/anacrolix/generics v0.1.0 // indirect
	github.com/anacrolix/log v0.17.0 // indirect
	github.com/anchore/go-lzo v0.1.0 // indirect
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/appscode/go-querystring v0.0.0-20170504095604-0126cfb3f1dc // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aws/aws-sdk-go-v2 v1.41.5 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.8 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.32.8 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.8 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.17 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.22.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.97.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.14 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.6 // indirect
	github.com/aws/smithy-go v1.24.2 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.6.1 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/boombuler/barcode v1.1.0 // indirect
	github.com/bradenaw/juniper v0.15.3 // indirect
	github.com/bradfitz/iter v0.0.0-20191230175014-e8f45d346db8 // indirect
	github.com/buengese/sgzip v0.1.1 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/calebcase/tmpfile v1.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chilts/sid v0.0.0-20190607042430-660e94789ec9 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/cloudinary/cloudinary-go/v2 v2.13.0 // indirect
	github.com/cloudsoda/go-smb2 v0.0.0-20250228001242-d4c70e6251cc // indirect
	github.com/cloudsoda/sddl v0.0.0-20250224235906-926454e91efc // indirect
	github.com/colinmarc/hdfs/v2 v2.4.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/creasty/defaults v1.8.0 // indirect
	github.com/cronokirby/saferith v0.33.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/diskfs/go-diskfs v1.7.0 // indirect
	github.com/dromara/dongle v1.0.1 // indirect
	github.com/dropbox/dropbox-sdk-go-unofficial/v6 v6.0.5 // indirect
	github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/emersion/go-message v0.18.2 // indirect
	github.com/emersion/go-vcard v0.0.0-20241024213814-c9703dde27ff // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/flynn/noise v1.1.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.11 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/gdamore/tcell/v2 v2.9.0 // indirect
	github.com/geoffgarside/ber v1.2.0 // indirect
	github.com/glebarez/go-sqlite v1.21.1 // indirect
	github.com/go-chi/chi/v5 v5.2.5 // indirect
	github.com/go-darwin/apfs v0.0.0-20211011131704-f84b94dbf348 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/errors v0.22.4 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/strfmt v0.25.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.28.0 // indirect
	github.com/go-resty/resty/v2 v2.16.5 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/safetext v0.0.0-20240104143208-7a7d9b3d812f // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.7 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gorilla/schema v1.4.1 // indirect
	github.com/hanwen/go-fuse/v2 v2.9.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/internxt/rclone-adapter v0.0.0-20260220172730-613f4cc8b8fd // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/goidentity/v6 v6.0.1 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jlaffaye/ftp v0.2.1-0.20240918233326-1b970516f5d3 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/jtolio/noiseconn v0.0.0-20231127013910-f6d9ecbf1de7 // indirect
	github.com/jzelinskie/whirlpool v0.0.0-20201016144138-0675e54bb004 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/koofr/go-httpclient v0.0.0-20240520111329-e20f8f203988 // indirect
	github.com/koofr/go-koofrclient v0.0.0-20221207135200-cbd7fc9ad6a6 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/lanrat/extsort v1.4.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lpar/date v1.0.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.21 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/mholt/archiver/v3 v3.5.1 // indirect
	github.com/mholt/archives v0.1.5 // indirect
	github.com/mikelolasagasti/xz v1.0.1 // indirect
	github.com/minio/minlz v1.0.1 // indirect
	github.com/minio/xxml v0.0.3 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/ncw/swift/v2 v2.0.5 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/nwaples/rardecode/v2 v2.2.1 // indirect
	github.com/oasdiff/yaml v0.0.9 // indirect
	github.com/oasdiff/yaml3 v0.0.12 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/oracle/oci-go-sdk/v65 v65.104.0 // indirect
	github.com/orca-zhang/ecache v1.1.3 // indirect
	github.com/panjf2000/ants/v2 v2.11.3 // indirect
	github.com/pengsrc/go-shared v0.2.1-0.20190131101655-1999055a4a14 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/peterh/liner v1.2.2 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.10 // indirect
	github.com/pkg/xattr v0.4.12 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/pquerna/otp v1.5.0 // indirect
	github.com/pressly/goose/v3 v3.27.1 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.2 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/putdotio/go-putio/putio v0.0.0-20200123120452-16d982cac2b8 // indirect
	github.com/rasky/go-xdr v0.0.0-20170124162913-1a41d1a06c93 // indirect
	github.com/rclone/Proton-API-Bridge v1.0.1-0.20260127174007-77f974840d11 // indirect
	github.com/rclone/go-proton-api v1.0.1-0.20260127173028-eb465cac3b18 // indirect
	github.com/rclone/gofakes3 v0.0.4 // indirect
	github.com/relvacode/iso8601 v1.7.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rfjakob/eme v1.1.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryszard/goskiplist v0.0.0-20150312221310-2dfbae5fcf46 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	github.com/shabbyrobe/gocovmerge v0.0.0-20230507112040-c3350d9342df // indirect
	github.com/shirou/gopsutil/v4 v4.25.10 // indirect
	github.com/sirupsen/logrus v1.9.4-0.20230606125235-dd1b4c2e81af // indirect
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966 // indirect
	github.com/smarty/assertions v1.15.0 // indirect
	github.com/sony/gobreaker v1.0.0 // indirect
	github.com/sorairolake/lzip-go v0.3.8 // indirect
	github.com/spacemonkeygo/monkit/v3 v3.0.25-0.20251022131615-eb24eb109368 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cobra v1.10.1 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/t3rm1n4l/go-mega v0.0.0-20251031123324-a804aaa87491 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/tyler-smith/go-bip39 v1.1.0 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/unknwon/goconfig v1.0.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/willscott/go-nfs v0.0.3 // indirect
	github.com/willscott/go-nfs-client v0.0.0-20251022144359-801f10d98886 // indirect
	github.com/winfsp/cgofuse v1.6.1-0.20260126094232-f2c4fccdb286 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/woodsbury/decimal128 v1.3.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	github.com/yunify/qingstor-sdk-go/v3 v3.2.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/assert v1.3.1 // indirect
	github.com/zeebo/blake3 v0.2.4 // indirect
	github.com/zeebo/errs v1.4.0 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.etcd.io/bbolt v1.4.3 // indirect
	go.mongodb.org/mongo-driver v1.17.6 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.68.0 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go4.org v0.0.0-20230225012048-214862532bf5 // indirect
	goftp.io/server/v2 v2.0.2 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/image v0.38.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/term v0.43.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/tools v0.44.0 // indirect
	google.golang.org/api v0.255.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260420184626-e10c466a9529 // indirect
	google.golang.org/grpc v1.80.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/validator.v2 v2.0.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.72.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.50.1 // indirect
	moul.io/http2curl/v2 v2.3.0 // indirect
	mvdan.cc/sh/v3 v3.7.0 // indirect
	storj.io/common v0.0.0-20251107171817-6221ae45072c // indirect
	storj.io/drpc v0.0.35-0.20250513201419-f7819ea69b55 // indirect
	storj.io/eventkit v0.0.0-20250410172343-61f26d3de156 // indirect
	storj.io/infectious v0.0.2 // indirect
	storj.io/picobuf v0.0.4 // indirect
	storj.io/uplink v1.13.1 // indirect
)

replace github.com/neochaotic/powerlab/backend/pkg => ../pkg

replace github.com/neochaotic/powerlab/backend/common => ../common
