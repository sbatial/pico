package pro

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/picosh/pico/shared"
	"github.com/stripe/stripe-go/v75"
)

func StartWebServer() {
	http.HandleFunc("/webhook", func(w http.ResponseWriter, req *http.Request) {
		const MaxBodyBytes = int64(65536)
		req.Body = http.MaxBytesReader(w, req.Body, MaxBodyBytes)
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading request body: %v\n", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		event := stripe.Event{}

		if err := json.Unmarshal(payload, &event); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse webhook body json: %v\n", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Unmarshal the event data into an appropriate struct depending on its Type
		switch event.Type {
		/* case "payment_intent.succeeded":
		var paymentIntent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &paymentIntent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Printf("%+v\n", paymentIntent)
		// Then define and call a func to handle the successful payment intent.
		// handlePaymentIntentSucceeded(paymentIntent) */
		case "checkout.session.completed":
			var checkout stripe.CheckoutSession
			err := json.Unmarshal(event.Data.Raw, &checkout)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			customerID := checkout.Customer.ID
			email := checkout.CustomerDetails.Email
			created := checkout.Created
			status := checkout.PaymentStatus
			picoUsername := ""
			for _, field := range checkout.CustomFields {
				if field.Key == "picousername" {
					picoUsername = field.Text.Value
					break
				}
			}
			log.Printf("customer ID: %s\n", customerID)
			log.Printf("email: %s\n", email)
			log.Printf("created at: %d\n", created)
			log.Printf("pico username: %s\n", picoUsername)
			log.Printf("payment status: %s\n", status)
		default:
			log.Printf("Unhandled event type: %s\n", event.Type)
		}

		w.WriteHeader(http.StatusOK)
	})

	port := shared.GetEnv("PRO_PORT", "3000")
	portStr := fmt.Sprintf(":%s", port)
	log.Printf("Starting web server on port %s", port)
	log.Fatal(http.ListenAndServe(portStr, nil))
}
