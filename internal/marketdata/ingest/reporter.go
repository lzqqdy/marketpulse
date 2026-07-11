package ingest

import "time"

// ProviderReporter records upstream provider health signals.
type ProviderReporter interface {
	ReportSuccess(name string, latency time.Duration)
	ReportFailure(name string, err error)
	ReportDisabled(name string)
	ReportUsed(name string, used bool)
}

// ProviderReporter returns the shared provider health store.
func (s *Service) ProviderReporter() ProviderReporter {
	return s.providerHealth
}
