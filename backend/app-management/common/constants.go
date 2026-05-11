package common

const (
	AppManagementServiceName = "app-management"
	AppManagementVersion     = "0.4.16"

	AppsDirectoryName = "Apps"

	// ComposeAppAuthorCasaOSTeam is the literal author string used by the
	// upstream CasaOS app store. Do not change — store apps still ship
	// with this value and we use it to identify their origin.
	ComposeAppAuthorCasaOSTeam = "CasaOS Team"
	// ComposeAppAuthorPowerLabTeam is what the PowerLab UI writes into
	// the `author:` field of newly authored compose manifests.
	ComposeAppAuthorPowerLabTeam = "PowerLab Team"

	// PowerLab canonical extension key. Newly authored compose files
	// should use this; older files using the legacy aliases below are
	// transparently translated by service.LookupAppExtension.
	ComposeExtensionNameXPowerLab = "x-powerlab"
	// Legacy aliases — read for compatibility, never authored fresh.
	// x-web was an intermediate name used briefly upstream;
	// x-casaos is the original CasaOS extension name and is what most
	// store apps still ship with today.
	ComposeExtensionNameXCasaOS                = "x-casaos"
	ComposeExtensionNameWeb                    = "x-web"
	ComposeExtensionPropertyNameStoreAppID     = "store_app_id"
	ComposeExtensionPropertyNameTitle          = "title"
	ComposeExtensionPropertyNameIsUncontrolled = "is_uncontrolled"

	ComposeYAMLFileName = "docker-compose.yml"

	DefaultCategoryFont = "grid"
	DefaultLanguage     = "en_us"
	DefaultPassword     = "powerlab"
	DefaultPGID         = "1000"
	DefaultPUID         = "1000"
	DefaultUserName     = "admin"

	Localhost           = "127.0.0.1"
	MIMEApplicationYAML = "application/yaml"

	CategoryListFileName  = "category-list.json"
	RecommendListFileName = "recommend-list.json"
)

// the tags can add more. like "latest", "stable", "edge", "beta", "alpha"
var NeedCheckDigestTags = []string{"latest"}
