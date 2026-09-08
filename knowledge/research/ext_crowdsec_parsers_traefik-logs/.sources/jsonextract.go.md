---
url: https://github.com/crowdsecurity/crowdsec/blob/master/pkg/exprhelpers/jsonextract.go
title: CrowdSec UnmarshalJSON helper
fetched: 2026-09-08
authority: source
ref: github.com/crowdsecurity/crowdsec@master:pkg/exprhelpers/jsonextract.go
---

```go
func UnmarshalJSON(params ...any) (any, error) {
	jsonBlob := params[0].(string)
	target := params[1].(map[string]interface{})
	key := params[2].(string)
	var out interface{}
	err := json.Unmarshal([]byte(jsonBlob), &out)
	if err != nil {
		log.WithField("line", jsonBlob).Errorf("UnmarshalJSON : %s", err)
		return nil, err
	}
	target[key] = out
	return nil, nil
}
```

No schema validation: any valid JSON value unmarshals. Third argument is only the map key under `evt.Unmarshaled`. On failure logs `UnmarshalJSON : <err>` at error level and returns error to the expr filter.
