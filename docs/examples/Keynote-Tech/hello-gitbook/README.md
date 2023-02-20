# Introduction

```go
func main() {
    fmt.Println("Hello, gitbook!")
}
```

```bash
$ cd Keynote-Tech/

$ gitbook init hello-gitbook

# add hello-gitbook
$ vi .folder.yaml

$ cd hello-gitbook

$ npm init -y

$ npm i gitbook-plugin-prism gitbook-plugin-code --save

$ rm package.json package-lock.json

$ cp ../../this-is-a-gitbook/book.json ./

$ gitbook build ./ latest
```