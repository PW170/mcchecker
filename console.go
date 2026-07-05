package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func init() {
	enableAnsi()
}

func enableAnsi() {
	handle := windows.Handle(os.Stdout.Fd())
	var mode uint32
	windows.GetConsoleMode(handle, &mode)
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	windows.SetConsoleMode(handle, mode)
}

type Console struct {
	reader *bufio.Reader
}

func NewConsole() *Console {
	return &Console{
		reader: bufio.NewReader(os.Stdin),
	}
}

func (c *Console) Clear() {
	fmt.Print("\033[H\033[2J")
}

func (c *Console) SetTitle(title string) {
	fmt.Printf("\033]0;%s\007", title)
}

func (c *Console) Println(a ...interface{}) {
	fmt.Println(a...)
}

func (c *Console) Printf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

func (c *Console) Print(a ...interface{}) {
	fmt.Print(a...)
}

func (c *Console) Prompt(text string) string {
	fmt.Print(text)
	input, err := c.reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}

func (c *Console) ReadLine() string {
	input, _ := c.reader.ReadString('\n')
	return strings.TrimSpace(input)
}
