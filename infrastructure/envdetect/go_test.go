package envdetect

import (
	"reflect"
	"testing"
)

func TestGoEnvironmentHelpersAndConstantDefaults(t *testing.T) {
	code := `package config
import "os"
const fallback = "mobile-example"
func clientID(envName, defaultID string) string {
 value := os.Getenv(envName)
 if value == "" { return defaultID }
 return value
}
func unrelated(name string) string { return name }
func config(dynamic string) {
 clientID("CLIENT_ID_MOBILE_INTROSPECT", fallback)
 clientID("CLIENT_ID_WEB_INTROSPECT", "web-example")
 clientID(dynamic, fallback)
 unrelated("NOT_AN_ENV")
 // clientID("COMMENT_ONLY", fallback)
 os.Getenv("DIRECT_ENV")
}`
	refs, err := (GoExtractor{}).Extract("config.go", code)
	if err != nil || !reflect.DeepEqual(refs.Names, []string{"CLIENT_ID_MOBILE_INTROSPECT", "CLIENT_ID_WEB_INTROSPECT", "DIRECT_ENV"}) || refs.Examples["CLIENT_ID_MOBILE_INTROSPECT"] != "mobile-example" || refs.Examples["CLIENT_ID_WEB_INTROSPECT"] != "web-example" {
		t.Fatalf("refs=%+v err=%v", refs, err)
	}
}
