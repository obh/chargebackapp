package workflows

import (
	"fmt"
	"time"

	"github.com/obh/chargebackapp/models"

	"go.temporal.io/sdk/log"

	"go.temporal.io/sdk/workflow"
)

const (
	TaskQueue = "chargeback-queue"
)

type ChargebackWFInput struct {
	Chargeback models.Chargeback
	Payment    models.Payment
	Merchant   models.Merchant
}

type ChargebackState struct {
	MerchantResponded bool
	WFInput           ChargebackWFInput
	Documents         map[string]interface{}
	MessageHistory    []string
}

type ChargebackResult = ChargebackState

type chargebackWorkflow struct {
	ChargebackState
	runId  string
	logger log.Logger
}

func ChargebackWorkflowId(chargebackId int) string {
	return fmt.Sprintf("chargeback:%d", chargebackId)
}

func newChargebackWorfklow(ctx workflow.Context, state *ChargebackState) *chargebackWorkflow {
	return &chargebackWorkflow{
		ChargebackState: *state,
		runId:           workflow.GetInfo(ctx).WorkflowExecution.RunID,
		logger:          workflow.GetLogger(ctx),
	}
}

func (w *chargebackWorkflow) pushStatus(ctx workflow.Context, status string) error {
	return workflow.UpsertSearchAttributes(
		ctx,
		map[string]interface{}{
			"chargeBackWorkflowStatus": status,
		},
	)
}

func (w *chargebackWorkflow) waitForMerchantResponse(ctx workflow.Context, input *ChargebackWFInput) (*MerchantResponseResult, error) {

	activityoptions := workflow.ActivityOptions{
		// Set Activity Timeout duration
		ScheduleToCloseTimeout: 10*time.Second + MerchantSubmissionPeriod,
		HeartbeatTimeout:       5 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, activityoptions)
	w.logger.Info("invoking emailMerchant with details:", input.Payment.ID, input.Payment.ID)
	err := emailMerchant(ctx, input)
	if err != nil {
		return &MerchantResponseResult{}, err
	}
	submission, err := waitForSubmission(ctx)

	result := MerchantResponseResult(*submission)
	return &result, err
}

func (w *chargebackWorkflow) reverseFunds(ctx workflow.Context, payment models.Payment) (models.Payment, error) {
	fmt.Println("demo code")
	return payment, nil
}

func (w *chargebackWorkflow) sendDisputeFailedMail(ctx workflow.Context, payment models.Payment, customer models.Customer) error {
	fmt.Println("demo code")
	return nil
}

func ChargebackProcess(ctx workflow.Context, input *ChargebackWFInput) (*ChargebackResult, error) {
	w := newChargebackWorfklow(
		ctx,
		&ChargebackState{
			WFInput:        *input,
			Documents:      make(map[string]interface{}),
			MessageHistory: make([]string, 10),
		})

	response, err := w.waitForMerchantResponse(ctx, input)
	if err != nil {
		return &w.ChargebackState, err
	}
	w.MerchantResponded = response.MerchantResponded
	w.logger.Info("back to merchant response: ", response.MerchantResponded)
	// if !w.MerchantResponded {
	// 	response, err := w.reverseFunds(ctx, input.Payment)
	// 	if err != nil {
	// 		return &w.ChargebackState, err
	// 	}
	// 	return &w.ChargebackState, w.sendDisputeFailedMail(ctx, response, input.Payment.Customer)
	// }

	return &w.ChargebackState, nil
}
