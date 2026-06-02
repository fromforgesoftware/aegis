package api

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// JSON:API types for the synthetic MFA operations.
const (
	ResourceTypeMFAEnrollment   resource.Type = "mfaEnrollments"
	ResourceTypeMFARecoveryCode resource.Type = "mfaRecoveryCodes"
	ResourceTypeMFAVerification resource.Type = "mfaVerifications"
	ResourceTypeStepUp          resource.Type = "stepUps"
)

// MFAEnrollRequestDTO starts TOTP enrolment.
type MFAEnrollRequestDTO struct {
	resource.RestDTO

	RAccountID    string `jsonapi:"attr,accountId"`
	RIssuer       string `jsonapi:"attr,issuer,omitempty"`
	RAccountLabel string `jsonapi:"attr,accountLabel,omitempty"`
}

// MFAEnrollDTO returns the shared secret + provisioning URI (shown once).
type MFAEnrollDTO struct {
	resource.RestDTO

	RSecret          string `jsonapi:"attr,secret"`
	RProvisioningURI string `jsonapi:"attr,provisioningUri"`
}

func MFAEnrollToDTO(secret, uri string) *MFAEnrollDTO {
	dto := &MFAEnrollDTO{RSecret: secret, RProvisioningURI: uri}
	dto.RType = ResourceTypeMFAEnrollment
	return dto
}

// MFACodeRequestDTO carries an account + a TOTP code (confirm/verify).
type MFACodeRequestDTO struct {
	resource.RestDTO

	RAccountID string `jsonapi:"attr,accountId"`
	RCode      string `jsonapi:"attr,code"`
}

// MFARecoveryCodesDTO returns freshly generated recovery codes (shown once).
type MFARecoveryCodesDTO struct {
	resource.RestDTO

	RRecoveryCodes []string `jsonapi:"attr,recoveryCodes"`
}

func MFARecoveryCodesToDTO(codes []string) *MFARecoveryCodesDTO {
	dto := &MFARecoveryCodesDTO{RRecoveryCodes: codes}
	dto.RType = ResourceTypeMFARecoveryCode
	return dto
}

// MFAVerificationDTO is a simple allow/deny verification result.
type MFAVerificationDTO struct {
	resource.RestDTO

	RValid bool `jsonapi:"attr,valid"`
}

func MFAVerificationToDTO(valid bool) *MFAVerificationDTO {
	dto := &MFAVerificationDTO{RValid: valid}
	dto.RType = ResourceTypeMFAVerification
	return dto
}

// StepUpRequestDTO asks for a step-up token given a fresh factor proof.
type StepUpRequestDTO struct {
	resource.RestDTO

	RAccountID string `jsonapi:"attr,accountId"`
	RFactor    string `jsonapi:"attr,factor"`
	RProof     string `jsonapi:"attr,proof"`
}

// StepUpDTO is the minted token, its assurance level, and expiry.
type StepUpDTO struct {
	resource.RestDTO

	RToken     string    `jsonapi:"attr,token"`
	RACR       string    `jsonapi:"attr,acr"`
	RExpiresAt time.Time `jsonapi:"attr,expiresAt"`
}

func StepUpToDTO(token, acr string, expiresAt time.Time) *StepUpDTO {
	dto := &StepUpDTO{RToken: token, RACR: acr, RExpiresAt: expiresAt}
	dto.RType = ResourceTypeStepUp
	return dto
}
