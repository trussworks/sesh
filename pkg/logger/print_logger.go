package logger

import "fmt"

type PrintLogger int

func NewPrintLogger() PrintLogger {
	return 0
}

func (l PrintLogger) LogSeshEvent(message string, metadata map[string]string) {
	fmt.Println("SESH: "+message, metadata)
}
