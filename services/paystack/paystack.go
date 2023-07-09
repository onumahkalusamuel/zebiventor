package paystack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/onumahkalusamuel/zebiventor/config"
)

const initURL = "https://api.paystack.co/transaction/initialize"
const verifyURL = "https://api.paystack.co/transaction/verify/%v"

var headers = config.BodyStructure{}

var client = &http.Client{}

func setHeaders() {
	headers = config.BodyStructure{
		"Authorization": "Bearer " + os.Getenv("PAYSTACK_SECRET_KEY"),
		"Content-Type":  "application/json",
	}
}

func CreatePaymentLink(params map[string]any) config.BodyStructure {

	setHeaders()

	marshaled, _ := json.Marshal(params)

	req, _ := http.NewRequest("POST", initURL, bytes.NewBuffer(marshaled))

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, _ := client.Do(req)

	var res any

	err := json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return config.BodyStructure{
			"success": "false",
			"message": err.Error(),
		}
	}

	decoded := res.(map[string]any)

	if decoded["status"] == false {
		return config.BodyStructure{
			"success": "false",
			"message": fmt.Sprint(decoded["message"]),
		}
	}

	d := decoded["data"].(map[string]interface{})

	paymentLink := d["authorization_url"]

	return config.BodyStructure{
		"success": "true",
		"link":    paymentLink.(string),
	}
}

func VerifyPayment(reference string) config.BodyStructure {

	setHeaders()

	url := fmt.Sprintf(verifyURL, reference)

	req, _ := http.NewRequest("GET", url, bytes.NewBuffer([]byte{}))

	req.Header.Set("Authorization", headers["Authorization"])

	resp, _ := client.Do(req)

	var res any

	err := json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return config.BodyStructure{
			"success": "false",
			"message": err.Error(),
		}
	}

	decoded := res.(map[string]any)

	if decoded["status"] == false {
		return config.BodyStructure{
			"success": "false",
			"message": fmt.Sprint(decoded["message"]),
		}
	}

	d := decoded["data"].(map[string]interface{})

	// check if it failed
	if d["status"].(string) != "success" {
		return config.BodyStructure{
			"success": "false",
			"message": d["gateway_response"].(string),
		}
	}

	metadata := d["metadata"].(map[string]interface{})

	if metadata["UserID"].(string) == "" ||
		metadata["BlockGroupID"].(string) == "" ||
		metadata["PaymentReference"].(string) == "" {
		return config.BodyStructure{
			"success": "false",
			"message": "Vital details missing. Please contact admin.",
		}
	}
	// successful response
	return config.BodyStructure{
		"success":          "true",
		"amount":           fmt.Sprint(int(d["amount"].(float64))),
		"userID":           metadata["UserID"].(string),
		"blockGroupID":     metadata["BlockGroupID"].(string),
		"paymentReference": metadata["PaymentReference"].(string),
	}
}

func GetCallbackURL(serverHost string) string {

	var protocol = "https://"

	if os.Getenv("ENV") == "dev" {
		protocol = "http://"
	}

	var PaystackCallBackURL = protocol + serverHost + "/account/paystack-callback"
	return PaystackCallBackURL
}
