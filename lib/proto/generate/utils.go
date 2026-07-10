package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-rod/rod/lib/utils"
	"github.com/ysmood/gson"
)

// Tip-of-tree protocol definitions, generated from Chromium main by the
// ChromeDevTools/devtools-protocol repo. Merged, these two files are the
// same content a browser serves at /json/protocol.
const (
	browserProtocolURL = "https://raw.githubusercontent.com/ChromeDevTools/devtools-protocol/master/json/browser_protocol.json"
	jsProtocolURL      = "https://raw.githubusercontent.com/ChromeDevTools/devtools-protocol/master/json/js_protocol.json"
)

func getSchema() gson.JSON {
	obj := gson.New(download(browserProtocolURL))
	js := gson.New(download(jsProtocolURL))

	domains := append(obj.Get("domains").Val().([]interface{}), js.Get("domains").Val().([]interface{})...)
	obj.Set("domains", domains)

	utils.E(utils.OutputFile("tmp/proto.json", obj.JSON("", "  ")))

	return obj
}

func download(u string) []byte {
	res, err := http.Get(u) //nolint: noctx
	utils.E(err)
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		panic("unexpected status " + res.Status + " downloading " + u)
	}

	data, err := io.ReadAll(res.Body)
	utils.E(err)

	return data
}

func mapType(n string) string {
	return map[string]string{
		"boolean": "bool",
		"number":  "float64",
		"integer": "int",
		"string":  "string",
		"binary":  "[]byte",
		"object":  "map[string]gson.JSON",
		"any":     "gson.JSON",
	}[n]
}

func typeName(domain *domain, schema gson.JSON) string {
	typeName := ""
	if schema.Has("type") {
		typeName = schema.Get("type").Str()
	}

	if typeName == "array" { //nolint: nestif
		item := schema.Get("items")

		if item.Has("type") {
			typeName = "[]" + mapType(item.Get("type").Str())
		} else {
			ref := item.Get("$ref").Str()
			if domain.ref(ref) {
				typeName = "[]*" + refName(domain.name, ref)
			} else {
				typeName = "[]" + refName(domain.name, ref)
			}
		}
	} else if schema.Has("$ref") {
		ref := schema.Get("$ref").Str()
		if domain.ref(ref) {
			typeName += "*"
		}
		typeName += refName(domain.name, ref)
	} else {
		typeName = mapType(typeName)

		// The devtools-protocol repo JSON lowers the pdl "binary" type to
		// "string" with this marker appended to the description; a browser's
		// /json/protocol serves the same fields as "binary". Restore []byte
		// so the JSON codec base64-encodes and decodes them.
		if typeName == "string" &&
			strings.Contains(schema.Get("description").Str(), "Encoded as a base64 string when passed over JSON") {
			typeName = "[]byte"
		}
	}

	switch typeName {
	case "NetworkTimeSinceEpoch", "InputTimeSinceEpoch":
		typeName = "TimeSinceEpoch"
	case "NetworkMonotonicTime":
		typeName = "MonotonicTime"
	}

	return typeName
}

func enumList(schema gson.JSON) []string {
	var enum []string
	if schema.Has("enum") {
		enum = []string{}
		for _, v := range schema.Get("enum").Arr() {
			if _, ok := v.Val().(string); !ok {
				panic("enum type error")
			}
			enum = append(enum, v.Str())
		}
	}

	return enum
}

func jsonTag(name string, optional bool) string {
	jsonTagValue := name
	if optional {
		jsonTagValue += ",omitempty"
	}
	return fmt.Sprintf("`json:\"%s\"`", jsonTagValue)
}

func refName(domain, id string) string {
	if strings.Contains(id, ".") {
		return symbol(id)
	}
	return domain + symbol(id)
}

// make sure golint works fine.
func symbol(n string) string {
	if n == "" {
		return ""
	}

	n = strings.ReplaceAll(n, ".", "")

	dashed := regexp.MustCompile(`[-_]`).Split(n, -1)
	if len(dashed) > 1 {
		converted := []string{}
		for _, part := range dashed {
			converted = append(converted, strings.ToUpper(part[:1])+part[1:])
		}
		n = strings.Join(converted, "")
	}

	n = strings.ToUpper(n[:1]) + n[1:]

	n = replaceLower(n, "Id")
	n = replaceLower(n, "Css")
	n = replaceLower(n, "Url")
	n = replaceLower(n, "Uuid")
	n = replaceLower(n, "Xml")
	n = replaceLower(n, "Http")
	n = replaceLower(n, "Dns")
	n = replaceLower(n, "Cpu")
	n = replaceLower(n, "Mime")
	n = replaceLower(n, "Json")
	n = replaceLower(n, "Html")
	n = replaceLower(n, "Guid")
	n = replaceLower(n, "Sql")
	n = replaceLower(n, "Eof")
	n = replaceLower(n, "Api")
	n = replaceLower(n, "Ui")
	n = replaceLower(n, "Https")

	n = strings.Replace(n, "Ids", "IDs", -1)

	return n
}

func replaceLower(n, word string) string {
	return regexp.MustCompile(word+`([A-Z-_]|$)`).ReplaceAllStringFunc(n, strings.ToUpper)
}

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}
