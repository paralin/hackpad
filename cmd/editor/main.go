package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall/js"

	"github.com/johnstarich/go-wasm/internal/promise"
	"github.com/johnstarich/go-wasm/log"
	"go.uber.org/atomic"
)

var (
	showLoading = atomic.NewBool(false)
	loadingElem js.Value
	consoleElem js.Value

	document = js.Global().Get("document")
)

func main() {
	app := document.Call("createElement", "div")
	app.Call("setAttribute", "id", "app")
	document.Get("body").Call("insertBefore", app, nil)

	app.Set("innerHTML", `
<h1>Go WASM Playground</h1>

<h3><pre>main.go</pre></h3>
<textarea></textarea>
<div class="controls">
	<button>build</button>
	<button>run</button>
	<button>fmt</button>
	<div class="loading-indicator"></div>
</div>
<div class="console">
	<h3>Console</h3>
	<pre class="console-output"></pre>
</div>
`)
	loadingElem = app.Call("querySelector", ".controls .loading-indicator")
	consoleElem = app.Call("querySelector", ".console-output")
	editorElem := app.Call("querySelector", "textarea")
	controlButtonElems := app.Call("querySelectorAll", ".controls button")

	controlButtons := make(map[string]js.Value)
	for i := 0; i < controlButtonElems.Length(); i++ {
		button := controlButtonElems.Index(i)
		name := button.Get("textContent").String()
		controlButtons[name] = button
	}
	controlButtons["build"].Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		runProcess("go", "build", "-v", ".")
		return nil
	}))
	controlButtons["run"].Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		runPlayground()
		return nil
	}))
	controlButtons["fmt"].Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		runProcess("go", "fmt", ".").Then(func(_ js.Value) interface{} {
			contents, err := ioutil.ReadFile("main.go")
			if err != nil {
				log.Error(err)
				return nil
			}
			editorElem.Set("value", string(contents))
			return nil
		})
		return nil
	}))

	editorElem.Call("addEventListener", "keydown", js.FuncOf(codeTyper))
	editorElem.Call("addEventListener", "input", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go edited(func() string {
			return editorElem.Get("value").String()
		})
		return nil
	}))

	if err := os.Mkdir("playground", 0700); err != nil {
		log.Error("Failed to make playground dir", err)
		return
	}
	if err := os.Chdir("playground"); err != nil {
		log.Error("Failed to switch to playground dir", err)
		return
	}
	cmd := exec.Command("go", "mod", "init", "playground")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Error("Failed to run go mod init", err)
		return
	}

	mainGoContents := `package main

import (
	"fmt"

	"github.com/johnstarich/go/datasize"
)

func main() {
	fmt.Println("Hello from WASM!", datasize.Gigabytes(4))
}
`
	editorElem.Set("value", mainGoContents)
	go edited(func() string { return mainGoContents })
	select {}
}

func runProcess(name string, args ...string) promise.Promise {
	resolve, reject, prom := promise.New()
	go func() {
		success := startProcess(name, args...)
		if success {
			resolve(nil)
		} else {
			reject(nil)
		}
	}()
	return prom
}

func startProcess(name string, args ...string) (success bool) {
	if !showLoading.CAS(false, true) {
		return false
	}
	loadingElem.Get("classList").Call("add", "loading")
	defer func() {
		showLoading.Store(false)
		loadingElem.Get("classList").Call("remove", "loading")
	}()

	stdout := newElementWriter(consoleElem, "")
	stderr := newElementWriter(consoleElem, "stderr")

	_, _ = stdout.WriteString(fmt.Sprintf("$ %s %s\n", name, strings.Join(args, " ")))

	cmd := exec.Command(name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Start()
	if err != nil {
		_, _ = stderr.WriteString("Failed to start process: " + err.Error() + "\n")
		return false
	}
	err = cmd.Wait()
	if err != nil {
		_, _ = stderr.WriteString(err.Error() + "\n")
	}
	return err == nil
}

func edited(newContents func() string) {
	err := ioutil.WriteFile("main.go", []byte(newContents()), 0600)
	if err != nil {
		log.Error("Failed to write main.go: ", err.Error())
		return
	}
}

func runPlayground() {
	runProcess("go", "build", "-v", ".").Then(func(_ js.Value) interface{} {
		return runProcess("./playground")
	})
}