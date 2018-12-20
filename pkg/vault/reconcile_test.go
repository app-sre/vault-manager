package vault

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type item struct {
	name string
	data string
}

func (i item) Name() string {
	return i.name
}

func (i item) Equals(iface interface{}) bool {
	iitem, ok := iface.(item)
	if !ok {
		return false
	}

	return i.name == iitem.name && i.data == iitem.data
}

func TestDiffItems(t *testing.T) {
	table := []struct {
		description string
		config      []item
		existing    []item
		toBeWritten []item
		toBeDeleted []item
	}{
		{
			description: "all nil args returns lists of len(0)",
			config:      nil,
			existing:    nil,
			toBeWritten: []item{},
			toBeDeleted: []item{},
		},
		{
			description: "all config created when nothing already exists",
			config:      []item{{"x", "x"}},
			existing:    []item{},
			toBeWritten: []item{{"x", "x"}},
			toBeDeleted: []item{},
		},
		{
			description: "already existing items are a no-op",
			config:      []item{{"x", "x"}},
			existing:    []item{{"x", "x"}},
			toBeWritten: []item{},
			toBeDeleted: []item{},
		},
		{
			description: "items with the same name get updated",
			config:      []item{{"x", "newdata"}},
			existing:    []item{{"x", "olddata"}},
			toBeWritten: []item{{"x", "newdata"}},
			toBeDeleted: []item{},
		},
		{
			description: "empty config deletes all",
			config:      []item{},
			existing:    []item{{"x", "x"}, {"y", "y"}},
			toBeWritten: []item{},
			toBeDeleted: []item{{"x", "x"}, {"y", "y"}},
		},
	}

	for _, tt := range table {
		t.Run(tt.description, func(t *testing.T) {
			toBeWritten, toBeDeleted := DiffItems(intoInterface(tt.config), intoInterface(tt.existing))
			require.Equal(t, tt.toBeWritten, outOfInterface(toBeWritten))
			require.Equal(t, tt.toBeDeleted, outOfInterface(toBeDeleted))
		})
	}
}

func intoInterface(xs []item) (items []Item) {
	items = make([]Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}

func outOfInterface(xs []Item) (items []item) {
	items = make([]item, 0)
	for _, x := range xs {
		items = append(items, x.(item))
	}

	return
}

func TestOptionsEqual(t *testing.T) {
	table := []struct {
		description string
		x, y        map[string]interface{}
		expected    bool
	}{
		{
			description: "nil equals nil",
			x:           nil,
			y:           nil,
			expected:    true,
		},
		{
			description: "same single key is equal",
			x:           map[string]interface{}{"x": "x"},
			y:           map[string]interface{}{"x": "x"},
			expected:    true,
		},
		{
			description: "nil equals map of len(0)",
			x:           nil,
			y:           map[string]interface{}{},
			expected:    true,
		},
		{
			description: "same values, but out of order",
			x:           map[string]interface{}{"x": "x", "y": "y"},
			y:           map[string]interface{}{"y": "y", "x": "x"},
			expected:    true,
		},
		{
			description: "former larger than latter is not equal",
			x:           map[string]interface{}{"x": "x", "y": "y"},
			y:           map[string]interface{}{"x": "x"},
			expected:    false,
		},
		{
			description: "latter larger than former is not equal",
			x:           map[string]interface{}{"x": "x"},
			y:           map[string]interface{}{"x": "x", "y": "y"},
			expected:    false,
		},
		{
			description: "ttl keys in minutes and seconds are equal",
			x:           map[string]interface{}{"x_ttl": "60s"},
			y:           map[string]interface{}{"x_ttl": "1m"},
			expected:    true,
		},
	}

	for _, tt := range table {
		t.Run(tt.description, func(t *testing.T) {
			require.Equal(t, tt.expected, OptionsEqual(tt.x, tt.y))
		})
	}
}
