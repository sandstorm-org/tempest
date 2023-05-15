package browsermain

import (
	"strings"
	"syscall/js"

	"zenhack.net/go/jsapi/streams"
	"zenhack.net/go/tea"
	"zenhack.net/go/tea/events"
	"zenhack.net/go/tea/vdom"
	"zenhack.net/go/tea/vdom/builder"
	"zenhack.net/go/tempest/internal/browser/intl"
	"zenhack.net/go/tempest/internal/common/types"
	"zenhack.net/go/util/maps"
	"zenhack.net/go/util/slices"
)

var (
	h = builder.H
)

type (
	a = builder.A
	e = builder.E
)

func t(l10n intl.L10N, f intl.L10NString, args ...string) vdom.VNode {
	return builder.T(l10n.Fmt(f, args...))
}

var dummyNode = h("div", a{"class": "dummy-node"}, nil)

func (m Model) pageTitle() string {
	switch m.CurrentFocus {
	case FocusOpenGrain:
		return "Tempest - " + m.Grains[m.FocusedGrain].Title
	case FocusGrainList:
		return "Tempest - Grains"
	case FocusApps:
		return "Tempest - Apps"
	default:
		return "Tempest"
	}
}

func (m Model) View(ms tea.MessageSender[Model]) vdom.VNode {
	// Hack: a bit gross to be setting the title imperatively in View;
	// maybe integrate something more declarative into go-tea. This
	// is the best place to do it for now though, since we know
	// the state is up to date wrt. what's going to end up on the page
	// too.
	js.Global().Get("document").Set("title", m.pageTitle())

	content := dummyNode
	session, loginReady := m.LoginSessions.Get()
	if !loginReady {
		content = t(m.L10N, "Loading...")
	} else if session.Err() != nil {
		// TODO: deferrentiate between disconnects/failures. Or maybe just
		// tweak the API to return all this info in-band?
		content = viewLoginForm(m.L10N, m.LoginForm, ms)
	} else {
		switch m.CurrentFocus {
		case FocusGrainList:
			kvs := maps.Items(m.Grains)
			slices.SortOn(kvs, func(kv maps.KV[types.GrainID, Grain]) string {
				return kv.Value.Title
			})
			var grainNodes []vdom.VNode
			for _, kv := range kvs {
				grainNodes = append(
					grainNodes,
					viewGrain(ms, kv.Key, kv.Value),
				)
			}
			content = h("ul", nil, nil, grainNodes...)
		case FocusApps:
			content = m.viewApps(ms)
		case FocusOpenGrain:
			if m.FocusedGrain == "" {
				content = t(m.L10N, "Placeholder; select a grain.")
			}
		default:
			panic("Unknown focus value")
		}
	}
	keys := maps.Keys(m.OpenGrains)
	slices.SortOn(keys, func(k types.GrainID) string {
		return m.Grains[k].Title
	})
	var activeGrainNodes []vdom.VNode
	for _, k := range keys {
		activeGrainNodes = append(
			activeGrainNodes,
			viewOpenGrain(m.L10N, ms, k, m.Grains[k], m.FocusedGrain == k),
		)
	}
	var iframes []vdom.VNode
	for _, id := range m.GrainDomOrder.Items {
		var vnode vdom.VNode
		if id == "" {
			vnode = dummyNode
		} else {
			vnode = viewGrainIframe(m, id)
		}
		iframes = append(iframes, vnode)
	}
	contentNodes := append([]vdom.VNode{content}, iframes...)

	mainUiNodes := []vdom.VNode{
		h("div", a{"class": "main-ui__main"}, nil,
			h("div", a{"class": "main-ui__sidebar"}, nil,
				h("h1", nil, nil,
					h("a",
						a{"href": "#"},
						e{"click": ms.Event(ChangeFocus{InitialFocus})},
						t(m.L10N, "Tempest"),
					),
				),
				h("nav", nil, nil, h("ul", a{"class": "nav-links"}, nil,
					h("li", a{"class": "nav-link"}, nil,
						h("a",
							a{"href": "#/apps"},
							e{"click": ms.Event(ChangeFocus{FocusApps})},
							t(m.L10N, "Apps"),
						),
					),
					h("li", a{"class": "nav-link"}, nil,
						h("a",
							a{"href": "#/grains"},
							e{"click": ms.Event(
								ChangeFocus{FocusGrainList},
							)},
							t(m.L10N, "Grains"),
						),
					),
				)),
				h("h2", nil, nil, t(m.L10N, "Grains")),
				h("nav", nil, nil,
					h("ul", a{"class": "nav-links"}, nil, activeGrainNodes...),
				),
			),
			h("div", a{"class": "main-ui__content"}, nil, contentNodes...),
		),
	}

	for _, e := range m.Errors {
		mainUiNodes = append(
			mainUiNodes,
			// TODO: figure out how translating the error should work.
			h("div", a{"class": "error-notice"}, nil, builder.T(e.Error())),
		)
	}

	return h("body", nil, nil,
		h("div", a{"class": "main-ui"}, nil, mainUiNodes...),
	)
}

func (m Model) viewApps(ms tea.MessageSender[Model]) vdom.VNode {
	var appItems []vdom.VNode
	for id, pkg := range m.Packages {
		manifest, err := pkg.Manifest()
		if err != nil {
			println("manifest: " + err.Error())
			continue
		}
		l10nTitle, err := manifest.AppTitle()
		if err != nil {
			println("appTitle: " + err.Error())
			continue
		}
		title, err := l10nTitle.DefaultText()
		if err != nil {
			println("defaultText: " + err.Error())
			continue
		}
		actions, err := manifest.Actions()
		if err != nil {
			println("actions: " + err.Error())
			continue
		}
		var links []vdom.VNode
		for i := 0; i < actions.Len(); i++ {
			action := actions.At(i)
			nounPhrasel10n, err := action.NounPhrase()
			if err != nil {
				println("nounPhrase: " + err.Error())
				continue
			}
			nounPhrase, err := nounPhrasel10n.DefaultText()
			if err != nil {
				println("nounPhrase.defaultText: " + err.Error())
				continue
			}

			links = append(
				links,
				h("li", nil, nil, h("a",
					a{"href": "#/grain/new"},
					e{
						"click": ms.Event(SpawnGrain{
							Index: i,
							PkgID: id,
						}),
					},
					// TODO: figure out how translation
					// should work for app-provided strings.
					builder.T(nounPhrase),
				)),
			)
		}
		appItems = append(
			appItems,
			h("li", nil, nil,
				// TODO: figure out how translation
				// should work for app-provided strings.
				builder.T(title),
				h("ul", nil, nil, links...),
			),
		)
	}
	onPkgChange := func(e vdom.Event) any {
		// TODO: give the input an id or class or something, to
		// make this robust if we ever add another file input somewhere:
		file := js.Global().Get("document").
			Call("querySelector", "input[type=file]").Get("files").Index(0)
		name := file.Get("name").String()
		size := file.Get("size").Int()
		reader := streams.ReadableStreamDefaultReader{
			Value: file.Call("stream").Call("getReader"),
		}
		ms.Send(NewAppPkgFile{
			Name:   name,
			Size:   size,
			Reader: reader,
		})
		return nil
	}
	return h("div", nil, nil,
		h("label",
			a{"for": "package"},
			nil,
			t(m.L10N, "upload new app")),
		h("input",
			a{"type": "file", "name": "package"},
			e{"change": &onPkgChange},
		),
		h("ul", nil, nil, appItems...),
	)
}

func (lf LoginForm) View(l10n intl.L10N, ms tea.MessageSender[Model]) vdom.VNode {
	submitAttrs := a{"type": "submit"}
	if lf.TokenSent {
		if lf.TokenInput == "" {
			submitAttrs["disabled"] = "disabled"
		}

		return h("div", nil, nil,
			h("label", a{"for": "token"}, nil,
				t(l10n, "Login token"),
			),
			h("input",
				a{"name": "token", "value": lf.TokenInput},
				e{
					"input": events.OnInput(func(value string) {
						ms.Send(EditEmailToken{NewValue: value})
					}),
				}),
			h("button",
				submitAttrs,
				e{"click": ms.Event(SubmitEmailToken{})},
				t(l10n, "Log In"),
			),
		)
	} else {
		if !strings.Contains(lf.EmailInput, "@") {
			// TODO: maybe check for a TLD too?
			submitAttrs["disabled"] = "disabled"
		}
		return h("div", nil, nil,
			h("label", a{"for": "address"}, nil,
				t(l10n, "Email address for login"),
			),
			h("input", a{
				"name":        "address",
				"placeholder": l10n.Fmt("e.g. alice@example.com"),
				"value":       lf.EmailInput,
			}, e{
				"input": events.OnInput(func(value string) {
					ms.Send(EditEmailLogin{NewValue: value})
				}),
			}),
			h("button",
				submitAttrs,
				e{"click": ms.Event(SubmitEmailLogin{})},
				t(l10n, "Send token"),
			),
		)
	}
}

func viewLoginForm(l10n intl.L10N, lf LoginForm, ms tea.MessageSender[Model]) vdom.VNode {
	return h("div", nil, nil,
		h("form", a{"action": "/login/dev", "method": "post"}, nil,
			h("label", a{"for": "name"}, nil,
				t(l10n, "Dev account login"),
			),
			h("input", a{
				"name":        "name",
				"placeholder": "e.g. Alice Dev Admin",
			}, nil),
			h("button", a{"type": "submit"}, nil, t(l10n, "Submit")),
		),
		lf.View(l10n, ms),
	)
}

func viewOpenGrain(l10n intl.L10N, ms tea.MessageSender[Model], id types.GrainID, grain Grain, isFocused bool) vdom.VNode {
	focusGrain := ms.Event(FocusGrain{ID: id})
	classes := "open-grain-tab"
	if isFocused {
		classes += " open-grain-tab--focused"
	} else {
		classes += " open-grain-tab--unfocused"
	}
	titleRow := h("div", a{"class": "open-grain-tab__title-row"}, nil,
		h("a",
			a{
				"href":  "#/grain/" + string(id),
				"class": "open-grain-tab__title",
			},
			e{"click": focusGrain},
			builder.T(grain.Title),
		),
		h("button",
			a{"class": "close-button"},
			e{"click": ms.Event(CloseGrain{ID: id})},
			t(l10n, "Close Grain"),
		),
	)
	return h("li", a{"class": classes}, nil, titleRow)
}

func viewGrain(ms tea.MessageSender[Model], id types.GrainID, grain Grain) vdom.VNode {
	return h("li", a{"class": "nav-link"}, nil,
		h("a",
			a{"href": "#/grain/" + string(id)},
			e{"click": ms.Event(FocusGrain{ID: id})},
			builder.T(grain.Title),
		),
	)
}

func viewGrainIframe(m Model, id types.GrainID) vdom.VNode {
	grain := m.Grains[id]
	grainUrl := m.ServerAddr.Subdomain("ui-" + grain.Subdomain)
	qv := grainUrl.Query()
	qv.Set("sandstorm-sid", grain.SessionToken)
	qv.Set("path", "/")
	grainUrl.Path = "/_sandstorm-init"
	grainUrl.RawQuery = qv.Encode()
	class := "grain-iframe"
	if m.CurrentFocus != FocusOpenGrain || m.FocusedGrain != id {
		class += " grain-iframe--inactive"
	}
	return h("iframe", a{
		"src":   grainUrl.String(),
		"class": class,
	}, nil)
}
