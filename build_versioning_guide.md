

To export and inject a version string into your Go-based CLI, declare an unassigned string variable (e.g., version) in your code. During the build process, use Go's linker flags (-ldflags) to set that variable's value dynamically, ensuring your binaries remain tied directly to your Git tags or release pipelines.

1. Declare the Version Variable
In your main package (or a dedicated cmd package), define a variable to hold the version.gopackage main

```
import "fmt"

// Declare the variable that will hold the version
var version = "dev"

func main() {
    // Or wire this up to your CLI library (e.g., Cobra,urfave/cli)
    fmt.Println("CLI Version:", version)
}
```

2. Build with Linker Flags
When compiling, use the -ldflags="-X main.version=1.0.0" flag. This tells the Go linker to replace the version variable in the main package with the specified value at build time.
```bash
go build -ldflags="-X main.version=1.0.0" -o mycli .
```

Use code with caution.
Note: If your variable is in a different package, replace main with the full import path of that package. For example: -ldflags="-X ://github.com".

3. Automating with Git Tags
Hardcoding versions manually is prone to errors. Instead, dynamically inject the latest Git tag into your builds.

Linux/macOS:

```
go build -ldflags="-X main.version=$(git describe --tags --always)" -o mycli .
```

