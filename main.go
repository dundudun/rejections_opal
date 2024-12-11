package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
)

type RejectionReasonStruct struct {
	XMLName xml.Name `xml:"rejectionReason"`
	Name    string   `xml:"name"`
}

type CriterionStruct struct {
	Name            string `xml:"name"`
	NegativeMeaning string `xml:"negativeMeaning"`
}

func openDictionary(dictionary, service string) (*os.File, error) {
	specific_path := fmt.Sprintf("./dictionaries/%s_%s.xml", service, dictionary)
	general_path := fmt.Sprintf("./dictionaries/%s.xml", dictionary)
	dict, err := os.Open(specific_path)
	if err != nil {
		if pathErr, ok := err.(*fs.PathError); ok {
			// add output here that wasn't found specific dict for this service and used general one
			fmt.Printf("%s (path \"%s\")\n", pathErr.Err, pathErr.Path)
			dict, err = os.Open(general_path)
			if err != nil {
				return nil, fmt.Errorf("%s.xml dictionary opening failure: %v", dictionary, err)
			}
		} else {
			return nil, fmt.Errorf("%s_%s.xml dictionary opening failure: %v", service, dictionary, err)
		}
	}
	return dict, nil
}

func decodeRejectReasonsFromDict(dict *os.File, rejections map[string]string) error {
	decoder := xml.NewDecoder(dict)
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "record" {
				for _, attr := range t.Attr {
					if attr.Name.Local == "recordVersionId" {
						var name string
						decoder.DecodeElement(&name, &t)
						//fmt.Printf("elem: %s\n", name)

						var data map[string]interface{}
						err := json.Unmarshal([]byte(name), &data)
						if err != nil {
							// return to output file info about that
							return fmt.Errorf("json parsing failure of %s: %v", dict.Name(), err)
						}

						if data["name"] == nil {
							continue
						}

						if _, ok := rejections[data["name"].(string)]; ok {
							rejections[data["name"].(string)] = attr.Value
							fmt.Printf("found reason record %s\n", attr.Value)
						}
					}
				}
			}
		}
	}
	return nil
}

func decodeNegativeMeaningsFromDict(dict *os.File, negMeanings map[CriterionStruct]string) error {
	decoder := xml.NewDecoder(dict)
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "record" {
				for _, attr := range t.Attr {
					if attr.Name.Local == "recordVersionId" {
						var jsonStr string
						decoder.DecodeElement(&jsonStr, &t)
						//fmt.Printf("elem: %s\n", name)

						var data map[string]interface{}
						err := json.Unmarshal([]byte(jsonStr), &data)
						if err != nil {
							// return to output file info about that
							return fmt.Errorf("json parsing failure of %s: %v", dict.Name(), err)
						}

						if data["negativeMeaning"] == nil || data["name"] == nil {
							continue
						}

						obj := CriterionStruct{Name: data["name"].(string), NegativeMeaning: data["negativeMeaning"].(string)}

						if _, ok := negMeanings[obj]; ok {
							negMeanings[obj] = attr.Value
							//fmt.Printf("found negMeaning record %s for '%.190s'\n", attr.Value, data["negativeMeaning"].(string))
						}
					}
				}
			}
		}
	}
	return nil
}

func main() {

	fileSystem := os.DirFS(".")

	text := `do
$$
declare
	rec record;
	key_type int8;
begin
	for rec in (select scheme from regadm.m_projects where scheme = 'kostgo') loop
		perform set_config('search_path', rec.scheme, true);
		raise info '%', rec.scheme;

		select key into key_type from d_ref_dependent_type where full_name like 'DocRefRejectReasonType';
		raise info 'key of DocRefRejectReasonType = %', key_type;
`

	fs.WalkDir(fileSystem, "reglaments", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Fatal(err)
		}
		if d.IsDir() {
			return nil
		}
		fmt.Println(path)

		service := strings.Split(d.Name(), ".")[0]
		fmt.Println(service)

		reglament, err := os.Open(path)
		if err != nil {
			log.Fatalf("reglament opening failure: %v", err)
		}
		defer reglament.Close()

		decoder := xml.NewDecoder(reglament)

		rejections := make(map[string]string)
		negMeanings := make(map[CriterionStruct]string)

		var rejectionReason string
		var crit CriterionStruct

		for {
			token, err := decoder.Token()
			if err != nil {
				break
			}

			switch t := token.(type) {
			case xml.StartElement:
				switch t.Name.Local {
				case "criterion":
					err = decoder.DecodeElement(&crit, &t)
					if err != nil {
						log.Fatalf("criterion parsing failure: %v", err)
					}
					//fmt.Printf("%v\n", crit)
				case "rejectionReason":
					var rej RejectionReasonStruct
					err = decoder.DecodeElement(&rej, &t)
					if err != nil {
						log.Fatalf("rejectionReason parsing failure: %v", err)
					}
					rejectionReason = rej.Name
					//fmt.Printf("%v\n", criteria)
				}
			case xml.EndElement:
				if t.Name.Local == "criteria" {
					//fmt.Printf("%s ------ %s\n", crit.Name, crit.NegativeMeaning)
					if rejectionReason != "" {
						if _, ok := rejections[rejectionReason]; !ok {
							rejections[rejectionReason] = "TRASH"
						}
					} else if crit.NegativeMeaning != "" {
						//fmt.Printf("%s ------ %s\n", crit.Name, crit.NegativeMeaning)
						if _, ok := negMeanings[crit]; !ok {
							negMeanings[crit] = "TRASH"
						}
					}

					rejectionReason = ""
					crit = CriterionStruct{}
				}
			}
		}

		rejectReasonDict, err := openDictionary("rejectReason", service)
		if err != nil {
			// add to output file
			log.Printf("couldn't open rejectReason dictionary: %v\n", err)
		} else {
			defer rejectReasonDict.Close()
			err = decodeRejectReasonsFromDict(rejectReasonDict, rejections)
			if err != nil {
				//write to output file error message
			}
		}

		negativeMeaningDict, err := openDictionary("reasonForSuccessDecision", service)
		if err != nil {
			// add to output file
			log.Printf("couldn't open reasonForSuccessDecision dictionary: %v\n", err)
		} else {
			defer negativeMeaningDict.Close()
			err = decodeNegativeMeaningsFromDict(negativeMeaningDict, negMeanings)
			if err != nil {
				//write to output file error message
			}
		}

		i := 0
		if len(rejections) != 0 {
			for rejectName, rejectId := range rejections {
				i++
				if len([]rune(rejectName)) >= 1000 {
					text += fmt.Sprintf("          -- %d characters in full_name, could be limit in db on that column in 1000 symbols\n        --", len([]rune(rejectName)))
				}

				if !strings.HasSuffix(text, "--") {
					text += "        "
					if rejectId == "TRASH" {
						text += "  --couldn't find myself, so you need to do that using name dictionary:\n        --"
					}
				}
				text += fmt.Sprintf("insert into d_ref_dependents (alias, code_kcr, dependent_type, full_name, name, is_draft, sys_status) values ('Opal%sRejReason%d', '%s', key_type, '%s','%s', 0, 0);\n",
					service, i, rejectId, rejectName, rejectName)
			}
			text += "\n"
		}
		if len(negMeanings) != 0 {
			for obj, negMeanId := range negMeanings {
				i++
				if len([]rune(obj.NegativeMeaning)) >= 1000 {
					text += fmt.Sprintf("          -- %d characters in full_name, could be limit in db on that column in 1000 symbols\n        ", len([]rune(obj.NegativeMeaning)))
				}

				if !strings.HasSuffix(text, "--") {
					text += "        "
					if negMeanId == "TRASH" {
						text += "  --couldn't find myself, so you need to do that. name in dictionary:\n        --"
					}
				}
				text += fmt.Sprintf("insert into d_ref_dependents (alias, code_kcr, dependent_type, full_name, name, is_draft, sys_status) values ('Opal%sRejMeaning%d', '%s', key_type, '%s','%s', 0, 0);\n",
					service, i, negMeanId, obj.NegativeMeaning, obj.NegativeMeaning)
			}
		}

		return nil
	})

	text += `    end loop;
end;
$$;`

	outputFile, err := os.Create("script_to_add_rejection_reasons.sql")
	if err != nil {
		log.Fatalf("output creation failure: %v", err)
	}
	defer outputFile.Close()
	_, err = outputFile.Write([]byte(text))
	if err != nil {
		log.Fatalf("writing output failure: %v", err)
	}
	//fmt.Println(text)
}

/*
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
<soap:Body>
<ns2:GetResponseResponse xmlns="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/basic/1.2" xmlns:ns2="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/1.2" xmlns:ns3="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/faults/1.2">
<ns2:ResponseMessage>
<ns2:Response xmlns:ns2="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/1.2" Id="SIGNED_BY_SMEV">
<SenderProvidedResponseData xmlns="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/1.2" xmlns:S="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ns2="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/basic/1.2" xmlns:ns3="http://www.w3.org/2004/08/xop/include" Id="SIGNED_BY_CONSUMER">
<ns2:MessagePrimaryContent>
<tns:KCRResponse xmlns:tns="urn://rostelekom.ru/KCR/1.0.6">
<tns:kcrService>
<tns:secondaryFields>
<tns:subServices>
<tns:profiling>
<tns:profilingSettings>
<tns:answers>
<tns:additionalInformations>
<tns:criteria>
<tns:rejectionReason>
<tns:id>
chardate here
*/
