package ssw

import (
	"fmt"
	"github.com/ilijamt/envwrap"
	"github.com/oleiade/reflections"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Config_Env(t *testing.T) {

	env := envwrap.NewStorage()
	defer env.ReleaseAll()

	envars := map[string]string{
		"SERVICE_TIMEOUT_MS": "100",
	}

	for k, v := range envars {
		t.Logf("Storing %s = %s", k, v)
		env.Store(k, v)
	}

	c := NewConfig()

	tests := []struct {
		env      string
		property string
	}{
		{"SERVICE_TIMEOUT_MS", "Timeout"},
	}

	for i, test := range tests {
		t.Logf("run test (%v): %v = %v", i, test.env, envars[test.env])
		value, _ := reflections.GetField(c, test.property)
		assert.EqualValues(t, envars[test.env], fmt.Sprintf("%v", value))
	}

}
