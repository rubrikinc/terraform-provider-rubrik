default: testacc

.PHONY: testacc testacc-aws testacc-azure testacc-gcp testacc-other

AWS_RUN    := ^TestAcc(Aws|PolarisAWS)
AZURE_RUN  := ^TestAcc(Azure|PolarisAzure)
GCP_RUN    := ^TestAcc(GCP|PolarisGCP)
OTHER_SKIP := ^TestAccCDM|^TestAcc(Aws|PolarisAWS|Azure|PolarisAzure|GCP|PolarisGCP)

testacc:
	TF_ACC=1 go test -count=1 -timeout=120m -v $(TESTARGS) ./...

testacc-aws:
	TF_ACC=1 go test '-run=$(AWS_RUN)' -count=1 -timeout=120m -v $(TESTARGS) ./...

testacc-azure:
	TF_ACC=1 go test '-run=$(AZURE_RUN)' -count=1 -timeout=120m -v $(TESTARGS) ./...

testacc-gcp:
	TF_ACC=1 go test '-run=$(GCP_RUN)' -count=1 -timeout=120m -v $(TESTARGS) ./...

testacc-other:
	TF_ACC=1 go test '-skip=$(OTHER_SKIP)' -count=1 -timeout=120m -v $(TESTARGS) ./...
