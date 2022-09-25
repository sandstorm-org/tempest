package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
)

type Config struct {
	User, Group   string
	Prefix        string
	ExecPrefix    string
	Bindir        string
	Libexecdir    string
	Localstatedir string
}

func (c *Config) ParseFlags(args []string, name string, errorHandling flag.ErrorHandling) {
	fs := flag.NewFlagSet(name, errorHandling)
	fs.StringVar(&c.User, "user", "sandstorm", "the user to run as")
	fs.StringVar(&c.Group, "group", "sandstorm", "the group to run as")

	fs.StringVar(&c.Prefix, "prefix", "/usr/local", "install prefix")
	fs.StringVar(&c.ExecPrefix, "exec-prefix", "", "executable prefix (default ${PREFIX})")
	fs.StringVar(&c.Bindir, "bindir", "", "path for executables (default ${EXEC_PREFIX}/bin)")
	fs.StringVar(&c.Libexecdir, "libexecdir", "",
		`path for helper commands (default "${PREFIX}/libexec")`)
	fs.StringVar(&c.Localstatedir, "localstatedir", "",
		`path to store run-time data (default "${PREFIX}/var/lib")`)

	// currently unused, but permitted, for compatibility with gnu coding guidelines/autoconf.
	fs.String("sbindir", "", "unused")
	fs.String("sysconfdir", "", "unused")
	fs.String("sharedstatedir", "", "unused")
	fs.String("runstatedir", "", "unused")
	fs.String("libdir", "", "unused")
	fs.String("includedir", "", "unused")
	fs.String("oldincludedir", "", "unused")
	fs.String("datarootdir", "", "unused")
	fs.String("datadir", "", "unused")
	fs.String("infodir", "", "unused")
	fs.String("mandir", "", "unused")
	fs.String("docdir", "", "unused")
	fs.String("htmldir", "", "unused")
	fs.String("dvidir", "", "unused")
	fs.String("pdfdir", "", "unused")
	fs.String("psdir", "", "unused")

	fs.Parse(args[1:])

	if c.ExecPrefix == "" {
		c.ExecPrefix = c.Prefix
	}
	if c.Bindir == "" {
		c.Bindir = c.ExecPrefix + "/bin"
	}
	if c.Libexecdir == "" {
		c.Libexecdir = c.Prefix + "/libexec"
	}
	if c.Localstatedir == "" {
		c.Localstatedir = c.Prefix + "/var/lib"
	}
}

func (c Config) GoSrc() string {
	return fmt.Sprintf(`package conifg

const (
	User = %q
	Group = %q
	Prefix = %q
	Libexecdir = %q
	Localstatedir = %q
)
`,
		c.User,
		c.Group,
		c.Prefix,
		c.Libexecdir,
		c.Localstatedir,
	)
}

func (c Config) CSrc() string {
	return fmt.Sprintf(`
#ifndef SANDSTORM_CONFIG_H
#define SANDSTORM_CONFIG_H

#define PREFIX %q
#define LIBEXECDIR %q
#define LOCALSTATEDIR %q

#endif
`,
		c.Prefix,
		c.Libexecdir,
		c.Localstatedir,
	)
}

func chkfatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func spawnErr(fn func() error) <-chan error {
	ret := make(chan error)
	go func() {
		ret <- fn()
	}()
	return ret
}

func withMyOuts(cmd *exec.Cmd) *exec.Cmd {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func runInDir(dir, bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	return withMyOuts(cmd).Run()
}

func installExe(exe, dir, caps string) {
	destDir := os.Getenv("DESTDIR")
	src, err := os.Open("./_build/" + exe)
	chkfatal(err)
	defer src.Close()
	dstPathDir := destDir + dir + "/"
	chkfatal(os.MkdirAll(dstPathDir, 0755))
	dstPath := dstPathDir + exe
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_RDWR, 0770)
	chkfatal(err)
	defer dst.Close()
	_, err = io.Copy(dst, src)
	chkfatal(err)
	if caps != "" {
		chkfatal(withMyOuts(exec.Command("setcap", caps, dstPath)).Run())
	}
}

func buildC() error {
	return runInDir("c", "make")
}

func buildGo() error {
	err := runInDir(".", "go", "build", "-v", "-o", "_build/sandstorm-next", "./go/cmd/sandstorm-next")
	if err != nil {
		return err
	}
	return runInDir(".", "go", "build", "-v", "-o", "_build/sandstorm-sandbox-agent", "./go/cmd/sandstorm-sandbox-agent")
}

func cleanC() error {
	return runInDir("c", "make", "clean")
}

func cleanGo() error {
	return runInDir(".", "rm", "-f", "_build/sandstorm-next")
}

func nukeC() error {
	return runInDir(".", "rm", "-f", "c/config.h")
}

func nukeGo() error {
	return runInDir(".", "rm", "-f", "go/internal/config/config.go")
}

// Run configure if its outputs aren't already present.
func maybeConfigure() {
	_, errC := os.Stat("./c/config.h")
	_, errGo := os.Stat("./go/internal/config/config.go")
	_, errJson := os.Stat("./config.json")
	if errC == nil && errGo == nil && errJson == nil {
		// Config is already present; we're done.
		return
	}
	log.Println("'configure' has not been run; running with default options.")
	run("configure")
}

func runJobs(jobs ...func() error) {
	chans := make([]<-chan error, len(jobs))
	errs := make([]error, len(jobs))
	for i := range jobs {
		chans[i] = spawnErr(jobs[i])
	}
	for i := range chans {
		errs[i] = <-chans[i]
	}
	for i := range errs {
		chkfatal(errs[i])
	}
}

func run(args ...string) {
	switch args[0] {
	case "build":
		maybeConfigure()
		runJobs(buildC, buildGo)
	case "run":
		run("build")
		fmt.Fprintln(os.Stderr, "Starting server...")
		chkfatal(withMyOuts(exec.Command("./bin/server")).Run())
	case "clean":
		maybeConfigure()
		runJobs(cleanC, cleanGo)
	case "nuke":
		run("clean")
		runJobs(nukeC, nukeGo)
		os.Remove("config.json")
	case "configure":
		cfg := &Config{}
		cfg.ParseFlags(args, "configure", flag.ExitOnError)
		chkfatal(ioutil.WriteFile(
			"./go/internal/config/config.go",
			[]byte(cfg.GoSrc()),
			0600))
		chkfatal(ioutil.WriteFile(
			"./c/config.h",
			[]byte(cfg.CSrc()),
			0600))
		jsonData, err := json.Marshal(cfg)
		chkfatal(err)
		chkfatal(ioutil.WriteFile(
			"./config.json",
			jsonData,
			0600))
	case "install":
		run("build")
		var c Config
		data, err := ioutil.ReadFile("config.json")
		chkfatal(err)
		chkfatal(json.Unmarshal(data, &c))
		installExe("sandstorm-next", c.Bindir, "cap_net_bind_service+ep")
		installExe("sandstorm-sandbox-launcher", c.Libexecdir+"/sandstorm", "cap_sys_admin+ep")
		installExe("sandstorm-sandbox-agent", c.Libexecdir+"/sandstorm", "")
	default:
		fmt.Fprintln(os.Stderr, "Unknown command:", args[0])
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) < 2 {
		run("build")
	} else {
		run(os.Args[1:]...)
	}
}
