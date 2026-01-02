package classifier

// ClassificationResult contains all classification results
type ClassificationResult struct {
	Disposable   bool
	RoleAccount  bool
	FreeProvider bool
}

// Classify performs all classifications on an email
func Classify(localPart, domain string) *ClassificationResult {
	return &ClassificationResult{
		Disposable:   IsDisposable(domain),
		RoleAccount:  IsRoleAccount(localPart),
		FreeProvider: IsFreeProvider(domain),
	}
}
