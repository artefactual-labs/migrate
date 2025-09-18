package main

import (
	"bufio"
	"fmt"
	"github.com/artefactual-labs/migrate/pkg/application"
	"os"
)

func main() {
	mapFilter := map[string]struct{}{}
	finalListSlice := []string{}

	var filterOutInput []string
	f, err := os.Open("to_filter_out.txt")
	application.PanicIfErr(err)
	s := bufio.NewScanner(f)
	toFilterCount := 0
	for s.Scan() {
		toFilterCount++
		mapFilter[s.Text()] = struct{}{}
		filterOutInput = append(filterOutInput, s.Text())
	}
	_, err = application.ValidateUUIDs(filterOutInput)
	if err != nil {
		application.PanicIfErr(err)
	}

	originalCount := 0
	{
		var input []string
		f, err := os.Open("original_list.txt")
		application.PanicIfErr(err)
		s := bufio.NewScanner(f)
		for s.Scan() {
			if _, ok := mapFilter[s.Text()]; !ok {
				finalListSlice = append(finalListSlice, s.Text())
			}
			input = append(input, s.Text())
			originalCount++
		}
		_, err = application.ValidateUUIDs(input)
		if err != nil {
			application.PanicIfErr(err)
		}
	}
	finalCount := 0
	{
		if err := os.Remove("final_list.txt"); err != nil {
			application.PanicIfErr(err)
		}

		f, err := os.OpenFile("final_list.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		application.PanicIfErr(err)
		for _, v := range finalListSlice {
			_, err := f.WriteString(v + "\n")
			application.PanicIfErr(err)
		}
		f.Close()
		{
			f, err := os.Open("final_list.txt")
			application.PanicIfErr(err)
			defer f.Close()
			s := bufio.NewScanner(f)
			for s.Scan() {
				finalCount++
			}
		}
	}
	fmt.Println("Original Count: ", originalCount)
	fmt.Println("To Filter Count: ", toFilterCount)
	fmt.Println("Final Count: ", finalCount)
}
