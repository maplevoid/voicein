{
  lib,
  buildGoModule,
  makeWrapper,
  wl-clipboard,
  wtype,
  xclip,
  xdotool,
  libnotify,
}:

let
  runtimeBins = [
    wl-clipboard
    wtype
    xclip
    xdotool
    libnotify
  ];
in
buildGoModule {
  pname = "voicein";
  version = "0.1.0";

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../cmd
      ../internal
      ../vendor
      ../go.mod
      ../go.sum
    ];
  };

  vendorHash = null;

  env.CGO_ENABLED = "0";

  nativeBuildInputs = [ makeWrapper ];

  subPackages = [ "cmd/voicein" ];

  checkPhase = ''
    runHook preCheck
    go test ./...
    runHook postCheck
  '';

  postFixup = ''
    wrapProgram $out/bin/voicein \
      --prefix PATH : ${lib.makeBinPath runtimeBins}
  '';

  meta = {
    description = "Push-to-toggle speech-to-text daemon for Linux";
    homepage = "https://github.com/maplevoid/voicein";
    mainProgram = "voicein";
    platforms = [
      "x86_64-linux"
      "aarch64-linux"
    ];
  };
}
