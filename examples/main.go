package main

import (
	"fmt"
	"io/ioutil"

	"github.com/jimexcel/excel"
	"github.com/jimexcel/mail"
)

func main() {
	files, err := excel.GetDirAllFiles("../fixtures/", ".eml")
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, f := range files {
		datas, err := ioutil.ReadFile(f)
		if err != nil {
			fmt.Println(err)
			return
		}

		msg, err := mail.ReadMessage(string(datas))
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println("Subject:", msg.Header.Subject())
		fmt.Println("From:", msg.Header.Addresses(mail.FromFieldName))
		fmt.Println("To:", msg.Header.Addresses(mail.ToFieldName))
	}
}
