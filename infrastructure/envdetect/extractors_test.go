package envdetect

import (
	domain "github.com/taupikpirdian/wlog/domain/environment"
	"reflect"
	"testing"
)

func extract(t *testing.T, path, code string) domain.References {
	t.Helper()
	var out domain.References
	for _, e := range DefaultExtractors() {
		if e.Supports(path) {
			r, err := e.Extract(path, code)
			if err != nil {
				t.Fatal(err)
			}
			out.Names = append(out.Names, r.Names...)
			out.CIOnly = append(out.CIOnly, r.CIOnly...)
		}
	}
	return out
}

func TestCrossLanguageLiteralPatterns(t *testing.T) {
	for _, tc := range []struct{ path, code string }{
		{"main.go", `os.Getenv("API_URL"); os.LookupEnv("API_URL")`},
		{"app.ts", `process.env.API_URL; process.env["API_URL"]; process.env['API_URL']; Deno.env.get("API_URL"); Bun.env.API_URL`},
		{"app.py", `os.getenv("API_URL"); os.environ.get("API_URL"); os.environ["API_URL"]`},
		{"app.php", `getenv('API_URL'); env('API_URL'); $_ENV['API_URL']; $_SERVER['API_URL']`},
		{"App.java", `System.getenv("API_URL"); @Value("${API_URL}"); environment.getProperty("API_URL")`},
		{"App.kt", `System.getenv("API_URL")`},
		{"App.cs", `Environment.GetEnvironmentVariable("API_URL")`},
		{"Configuration.cs", `var configuration = new ConfigurationBuilder().AddEnvironmentVariables().Build(); configuration["API_URL"]`},
		{"app.rb", `ENV["API_URL"]; ENV.fetch("API_URL")`},
		{"app.rs", `std::env::var("API_URL"); std::env::var_os("API_URL")`},
		{"deploy.sh", "export API_URL=private-value\necho $API_URL ${API_URL}\n"},
		{"Dockerfile", "ENV API_URL=private-value\nARG API_URL\nRUN echo $API_URL\n"},
		{"compose.yaml", "services:\n  api:\n    environment:\n      API_URL: ${API_URL}\n"},
		{"docker-compose.yml", "services:\n  api:\n    environment:\n      - API_URL=${API_URL}\n"},
		{"deployment.yaml", "spec:\n  containers:\n    - env:\n        - name: API_URL\n          valueFrom:\n            secretKeyRef:\n              name: app-secrets\n              key: API_URL\n"},
		{".env.example", "API_URL=private-value\nAPI_URL=other-value\n"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := extract(t, tc.path, tc.code).Names; !reflect.DeepEqual(got, []string{"API_URL"}) {
				t.Fatalf("got=%v", got)
			}
		})
	}
}

func TestNoDynamicGuessesOrGenericConfigFalsePositives(t *testing.T) {
	for _, tc := range []struct{ path, code string }{
		{"app.go", `os.Getenv(variableName); // os.Getenv("COMMENT_ONLY")`},
		{"concat.go", `os.Getenv("API_" + variableName)`},
		{"app.js", `process.env[key]; config["timeout"]; map["name"]; const doc = "process.env.DOC_ONLY"; // process.env.COMMENT_ONLY`},
		{"App.cs", `configuration["TIMEOUT"]; settings["HOST"]`},
		{"app.py", `os.environ[key] # os.getenv("COMMENT_ONLY")`},
		{"script.sh", "echo $LOCAL ${LOCAL}\nlocal_variable=abc\n"},
		{"compose.yaml", "# ${COMMENT_ONLY}\nservices: {}\n"},
		{"unknown.txt", `process.env.API_URL`},
		{"node_modules/app.js", `process.env.API_URL`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := extract(t, tc.path, tc.code); len(got.Names) > 0 {
				t.Fatalf("false positive=%v", got)
			}
		})
	}
}

func TestCIAndConfigMapNamesOnly(t *testing.T) {
	refs := extract(t, ".github/workflows/build.yml", "env:\n  BUILD_TOKEN: never-print-this-value\n")
	if len(refs.Names) != 0 || !reflect.DeepEqual(refs.CIOnly, []string{"BUILD_TOKEN"}) {
		t.Fatalf("refs=%+v", refs)
	}
	refs = extract(t, "secret.yaml", "kind: Secret\nmetadata:\n  name: resource-name\nstringData:\n  CLIENT_SECRET: never-print-this-value\n")
	if !reflect.DeepEqual(refs.Names, []string{"CLIENT_SECRET"}) {
		t.Fatalf("refs=%+v", refs)
	}
}

func TestInvalidYamlReturnsErrorWithoutPanicking(t *testing.T) {
	if _, err := (YAMLExtractor{}).Extract("compose.yml", "environment: [broken"); err == nil {
		t.Fatal("invalid YAML accepted")
	}
}
