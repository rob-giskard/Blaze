@echo off
rem Builds Blaze.exe (needs Go 1.22+ from https://go.dev/dl/)
go build -trimpath -ldflags "-s -w -H=windowsgui" -o Blaze.exe .
if %errorlevel%==0 echo Built Blaze.exe
