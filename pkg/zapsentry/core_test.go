package zapsentry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStackTrace(t *testing.T) {
	var res = newStackTrace()
	assert.Nil(t, res)

	ModuleIgnore = []string{}
	res = newStackTrace()
	assert.Len(t, res.Frames, 2)
}
