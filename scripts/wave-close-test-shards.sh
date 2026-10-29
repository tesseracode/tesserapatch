#!/bin/sh
set -eu

BASH_ENV=/dev/null
GOFLAGS=
GOENV=off
GOMAXPROCS=1
export BASH_ENV GOFLAGS GOENV GOMAXPROCS

go test -p=1 ./... -count=1 -timeout 40m -skip '^Test(S6[A-Z].*|S7[A-Z].*|AVP.*)$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestAVP.*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS6[A-F].*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS6[G-L].*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS6[M-R].*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS6[S-Z].*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS7A[A-O].*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS7AP.*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS7AQ.*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS7AS.*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS7AT.*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS7AU.*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS7AV.*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS7AW.*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS7AX.*$'
go test -p=1 ./... -count=1 -timeout 40m -run '^TestS7(A([YZ]|[^A-Z]|$)|[B-Z]).*$' -skip '^(TestS7AR.*|TestS7Observed(AMThroughAO|AP|AQ|AR(Core|Purge|Claims)|AS|AT|AU|AV|AW|AX)RegistrationAuthority)$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARRev(11|12|13|14|15).*$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARRev16.*$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARRev17.*$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARRev18.*$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARRev(19|20).*$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARRev(21|23|24).*$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARRev(25|26|27).*$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7AR(ExitSixRouteGuard|PrepareGrammarGuard|DivergenceContracts|AbandonContracts|ArchiveControlContracts|CoverageLedger|CoverageLedgerRejectsEmptyTarget)$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARAbandonGateTableGuard$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARPurgeProgressGuard$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARPermanentBlockClaimsGuard$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedARCoreRegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedARPurgeRegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedARClaimsRegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedAMThroughAORegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedAPRegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedAQRegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedASRegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedATRegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedAURegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedAVRegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedAWRegistrationAuthority$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ObservedAXRegistrationAuthority$'
