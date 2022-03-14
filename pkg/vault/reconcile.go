package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// Item represents a remote value stored in a Vault instance.
type Item interface {
	Key() string
	Equals(interface{}) bool
	KeyForDescription() string
	KeyForType() string
}

// DiffItems is a pure function that determines what changes need to be made to
// a Vault instance in order to reach the desired state.
func DiffItems(desired, existing []Item) (toBeWritten, toBeDeleted, toBeUpdated []Item) {
	toBeWritten = make([]Item, 0)
	toBeDeleted = make([]Item, 0)
	toBeUpdated = make([]Item, 0)

	existingNames := []string{}
	for _, existingItem := range existing {
		existingNames = append(existingNames, existingItem.Key())
	}

	if len(existing) == 0 && len(desired) != 0 {
		toBeWritten = desired
	} else {
		for _, item := range desired {

			if !in(item, existing) {
				if !deepComparisonForName(item.Key(), existingNames) {
					toBeWritten = append(toBeWritten, item)
				} else if !keyDescription(item, existing) && item.KeyForType() == "kv" {
					toBeUpdated = append(toBeUpdated, item)
				} else {
					toBeWritten = append(toBeWritten, item)
				}
			} else if in(item, existing) && !keyDescription(item, existing) && item.KeyForType() == "kv" {
				toBeUpdated = append(toBeUpdated, item)
			}

		}

		for _, item := range existing {
			if !keyIn(item, desired) {
				toBeDeleted = append(toBeDeleted, item)
			}
		}
	}

	return
}

func in(y Item, xs []Item) bool {
	for _, x := range xs {
		if y.Equals(x) {
			return true
		}
	}
	return false
}

func deepComparisonForName(y string, xs []string) bool {
	for _, x := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func keyIn(y Item, xs []Item) bool {
	for _, x := range xs {
		if y.Key() == x.Key() {
			return true
		}
	}
	return false
}

func keyDescription(y Item, xs []Item) bool {
	for _, x := range xs {
		if y.KeyForDescription() == x.KeyForDescription() {
			return true
		}
	}
	return false
}

// OptionsEqual compares two sets of options mappings.
func OptionsEqual(xopts, yopts map[string]interface{}) bool {
	if len(xopts) != len(yopts) {
		return false
	}

	for k, v := range yopts {
		xv, ok := xopts[k]
		if !ok {
			return false
		}

		// option values that need to be processed as numbers
		if strings.HasSuffix(k, "ttl") || strings.HasSuffix(k, "period") ||
			strings.HasSuffix(k, "leeway") || k == "max_age" {
			if !ttlEqual(fmt.Sprintf("%v", v), fmt.Sprintf("%v", xv)) {
				return false
			}
			continue
		} else if k == "bound_claims" || k == "claim_mappings" {
			mapped := UnmarshalJsonObj(k, xv)
			if reflect.DeepEqual(mapped, v) {
				continue
			}
			return false
		}

		if fmt.Sprintf("%v", v) != fmt.Sprintf("%v", xv) {
			return false
		}
	}

	return true
}

func ttlEqual(x, y string) bool {
	if x == y {
		return true
	}

	xdur, xerr := ParseDuration(x)
	ydur, yerr := ParseDuration(y)

	if xerr != nil || yerr != nil {
		return false
	}

	return xdur.Nanoseconds() == ydur.Nanoseconds()
}

// EqualPathNames determines if two paths are the same.
func EqualPathNames(x, y string) bool {
	return strings.Trim(x, "/") == strings.Trim(y, "/")
}

// ParseDuration parses a string duration from Vault.
// Defaults to seconds if no unit is found at the end of the string.
func ParseDuration(duration string) (time.Duration, error) {
	lastChar := string([]rune(duration)[len(duration)-1])
	if strings.ContainsAny(lastChar, "1234567890") {
		duration += "s"
	}

	return time.ParseDuration(duration)
}

// DataInSecret compare given data with data stored in the vault secret
func DataInSecret(data map[string]interface{}, path string) bool {
	// read desired secret
	secret := ReadSecret(path)
	if secret == nil {
		return false
	}
	for k, v := range data {
		if strings.HasSuffix(k, "ttl") || strings.HasSuffix(k, "period") {
			dur, err := ParseDuration(v.(string))
			if err != nil {
				log.WithError(err).WithField("option", k).Fatal("failed to parse duration from data")
			}
			v = int64(dur.Seconds())
		} else if k == "oidc_client_secret" { // not returned from ReadSecret()
			continue
		}

		if fmt.Sprintf("%v", secret.Data[k]) == fmt.Sprintf("%v", v) {
			continue
		}
		return false
	}
	return true
}

// Yaml unmarshal limitation causes nested options objects to be decode as strings with json format
// ex: `{"foo": "bar"}`
// UnmarshalJsonObj performs unmarshal of jsons strings
func UnmarshalJsonObj(key string, obj interface{}) map[string]interface{} {
	var x map[string]interface{}
	strObj, ok := obj.(string)
	if !ok {
		log.WithError(errors.New(fmt.Sprintf("Type conversion failed for %s", key)))
		return nil
	}
	err := json.Unmarshal([]byte(strObj), &x)
	if err != nil {
		log.WithError(err)
		return nil
	}
	return x
}
