# Introduction

```go
func main() {
    fmt.Println("Hello, gitbook!")
}
```

安装 node v12.22.12
安装 gitbook-cli

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