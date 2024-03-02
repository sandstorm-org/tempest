package session

import (
	"capnproto.org/go/capnp/v3/schemas"
	"sandstorm.org/go/tempest/internal/capnp/cookie"
)

func init() {
	cookie.RegisterSchema(schemas.DefaultRegistry)
}
