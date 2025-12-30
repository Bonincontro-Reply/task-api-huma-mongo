@echo off
setlocal EnableExtensions

set "CLUSTER_NAME=task-api"
set "RELEASE_NAME=task-api-huma-mongo"
set "NAMESPACE=task-api"
set "DELETE_CLUSTER=true"

echo Stopping kubectl port-forwards...
powershell -NoProfile -Command "Get-CimInstance Win32_Process -Filter \"Name='kubectl.exe'\" | Where-Object { $_.CommandLine -match 'port-forward' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue; Write-Output \"Stopped port-forward PID $($_.ProcessId)\" }" 2>NUL

where kubectl >NUL 2>NUL
if %errorlevel%==0 (
  where helm >NUL 2>NUL
  if %errorlevel%==0 (
    echo Uninstalling Helm release...
    helm uninstall %RELEASE_NAME% -n %NAMESPACE% >NUL 2>NUL
  ) else (
    echo helm not found, skipping release cleanup.
  )
  echo Deleting TaskSeed resources...
  kubectl -n %NAMESPACE% delete taskseed --all >NUL 2>NUL
  echo Deleting namespace "%NAMESPACE%"...
  kubectl delete namespace %NAMESPACE% --ignore-not-found >NUL 2>NUL
) else (
  echo kubectl not found, skipping resource cleanup.
)

where kind >NUL 2>NUL
if %errorlevel%==0 (
  if /I "%DELETE_CLUSTER%"=="true" (
    echo Deleting kind cluster "%CLUSTER_NAME%"...
    kind delete cluster --name %CLUSTER_NAME% >NUL 2>NUL
  ) else (
    echo Cluster delete disabled.
  )
) else (
  echo kind not found, skipping cluster delete.
)

echo Cleanup complete.
endlocal
