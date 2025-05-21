$link = "${env:VSINSTALLDIR}VC\Tools\MSVC\*\bin\Hostx64\x64\link.exe"
$link = Get-Command $link | Select-Object -First 1
& $link /DLL /NOENTRY /DEF:postgres.def /OUT:../output/postgres.exe