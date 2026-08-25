{
  lib,
  buildGoModule,
  stdenv,
  fetchurl,
  unzip,
  autoPatchelfHook,
  patchelf,
  makeWrapper,
  wl-clipboard,
  wtype,
  xclip,
  xdotool,
  libnotify,
}:

let
  sherpaLinuxVersion = "1.13.6";
  sherpaTriple =
    {
      x86_64-linux = "x86_64-unknown-linux-gnu";
      aarch64-linux = "aarch64-unknown-linux-gnu";
    }
    .${stdenv.hostPlatform.system};

  # go mod vendor drops the prebuilt .so files. The module zip from the
  # Go proxy still has them; copy the current arch into vendor before CGO.
  sherpaLinuxZip = fetchurl {
    url = "https://proxy.golang.org/github.com/k2-fsa/sherpa-onnx-go-linux/@v/v${sherpaLinuxVersion}.zip";
    hash = "sha256-pJfsHdMS7XaWd1wv5chuPS6YfTuD8ky5VPzJ831oD1A=";
  };

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

  env.CGO_ENABLED = "1";

  nativeBuildInputs = [
    unzip
    autoPatchelfHook
    patchelf
    makeWrapper
  ];

  buildInputs = [ stdenv.cc.cc.lib ];

  subPackages = [ "cmd/voicein" ];

  checkPhase = ''
    runHook preCheck
    export LD_LIBRARY_PATH="${lib.makeLibraryPath [ stdenv.cc.cc.lib ]}:$LD_LIBRARY_PATH"
    go test ./...
    runHook postCheck
  '';

  preBuild = ''
    mkdir -p vendor/github.com/k2-fsa/sherpa-onnx-go-linux/lib
    unzip -q ${sherpaLinuxZip} "*/lib/${sherpaTriple}/*"
    cp -a "github.com/k2-fsa/sherpa-onnx-go-linux@v${sherpaLinuxVersion}/lib/${sherpaTriple}" \
      vendor/github.com/k2-fsa/sherpa-onnx-go-linux/lib/
    rm -rf github.com
  '';

  postInstall = ''
    mkdir -p $out/lib
    cp -a vendor/github.com/k2-fsa/sherpa-onnx-go-linux/lib/${sherpaTriple}/*.so $out/lib/
    patchelf --set-rpath "$out/lib:${lib.makeLibraryPath [ stdenv.cc.cc.lib ]}" \
      $out/bin/voicein
  '';

  postFixup = ''
    wrapProgram $out/bin/voicein \
      --prefix PATH : ${lib.makeBinPath runtimeBins}
  '';

  meta = {
    description = "Push-to-toggle speech-to-text daemon for Niri";
    homepage = "https://github.com/maplevoid/voicein";
    mainProgram = "voicein";
    platforms = [
      "x86_64-linux"
      "aarch64-linux"
    ];
  };
}
