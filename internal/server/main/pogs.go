package servermain

import (
	"capnproto.org/go/capnp/v3/schemas"
	websession "sandstorm.org/go/tempest/capnp/web-session"
)

func init() {
	websession.RegisterSchema(schemas.DefaultRegistry)
}
