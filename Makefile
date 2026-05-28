default: testacc

.PHONY: testacc testacc-aws testacc-azure testacc-gcp testacc-other

AWS_RUN    := (?i)^TestAcc(Aws|PolarisAWS)
AZURE_RUN  := (?i)^TestAcc(Azure|PolarisAzure)
GCP_RUN    := (?i)^TestAcc(GCP|PolarisGCP)
OTHER_SKIP := (?i)^TestAccCDM|(?i)^TestAcc(Aws|PolarisAWS|Azure|PolarisAzure|GCP|PolarisGCP)

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
