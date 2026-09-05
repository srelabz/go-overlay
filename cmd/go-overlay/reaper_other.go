//go:build !linux || !(amd64 || arm64)

package main

func startZombieReaper() {}
