#!/bin/sh
set -eu

BASH_ENV=/dev/null
GOFLAGS=
GOENV=off
GOMAXPROCS=1
export BASH_ENV GOFLAGS GOENV GOMAXPROCS

go test -p=1 ./... -count=1 -timeout 40m -skip '^(TestS7AR.*|TestS7Observed(AMThroughAO|AP|AQ|AR(Core|Purge|Claims)|AS|AT|AU|AV|AW|AX)RegistrationAuthority)$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARRev(11|12|13|14|15).*$'
go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestS7ARRev(16|17|18).*$'
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
