{ pkgs ? import ./pkgs.nix {} }:

let src = import ./src.nix;

	shell = import "${src.gotk4}/.nix/shell.nix" {
		inherit pkgs;
	};

in shell
