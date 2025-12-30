@echo off
setlocal EnableExtensions EnableDelayedExpansion

set "CLUSTER_NAME=task-api"
set "SCRIPT_DIR=%~dp0"
set "PROJECT_ROOT=%SCRIPT_DIR%.."
set "RELEASE_NAME=task-api-huma-mongo"
set "NAMESPACE=task-api"
set "CHART_DIR=%PROJECT_ROOT%\deploy\helm\task-api-huma-mongo"
set "CHART_NAME=task-api-huma-mongo"
set "APPLY_SEED=true"
set "SEED_MANIFEST=%PROJECT_ROOT%\deploy\examples\taskseed-sample.yaml"
set "SEED_NAME=sample-seed"
set "SEED_TIMEOUT_SEC=120"

call :require docker
call :require kind
call :require kubectl
call :require helm

if not exist "%CHART_DIR%" (
  echo Missing Helm chart: %CHART_DIR%
  exit /b 1
)

echo Cleaning existing kind cluster "%CLUSTER_NAME%" (if any)...
for /f "delims=" %%c in ('kind get clusters 2^>NUL') do (
  if /I "%%c"=="%CLUSTER_NAME%" (
    kind delete cluster --name %CLUSTER_NAME% >NUL
  )
)

echo Creating kind cluster "%CLUSTER_NAME%"...
kind create cluster --name %CLUSTER_NAME% >NUL
if errorlevel 1 exit /b 1

echo Building images...
pushd "%PROJECT_ROOT%"
docker build -t task-api-huma-mongo:local -f deploy\docker\api.Dockerfile . >NUL
if errorlevel 1 exit /b 1
docker build -t task-api-huma-mongo-frontend:local -f deploy\docker\frontend.Dockerfile . >NUL
if errorlevel 1 exit /b 1

echo Loading images into kind...
kind load docker-image task-api-huma-mongo:local --name %CLUSTER_NAME% >NUL
if errorlevel 1 exit /b 1
kind load docker-image task-api-huma-mongo-frontend:local --name %CLUSTER_NAME% >NUL
if errorlevel 1 exit /b 1

echo Installing Helm chart...
helm upgrade --install %RELEASE_NAME% "%CHART_DIR%" --namespace %NAMESPACE% --create-namespace --wait --timeout 120s >NUL
if errorlevel 1 exit /b 1

set "FULL_NAME=%RELEASE_NAME%"
echo %RELEASE_NAME% | findstr /I /C:"%CHART_NAME%" >NUL
if errorlevel 1 set "FULL_NAME=%RELEASE_NAME%-%CHART_NAME%"

set "FRONTEND_SVC=%FULL_NAME%-frontend"
set "API_SVC=%FULL_NAME%-api"
set "MONGO_SVC=%FULL_NAME%-mongodb"
set "SEED_CONTROLLER=%FULL_NAME%-seed-controller"

kubectl get svc -n %NAMESPACE% %FRONTEND_SVC% >NUL 2>NUL
if errorlevel 1 (
  echo Could not find frontend service: %FRONTEND_SVC%
  exit /b 1
)
kubectl get svc -n %NAMESPACE% %API_SVC% >NUL 2>NUL
if errorlevel 1 (
  echo Could not find api service: %API_SVC%
  exit /b 1
)
kubectl get svc -n %NAMESPACE% %MONGO_SVC% >NUL 2>NUL
if errorlevel 1 (
  echo Could not find mongodb service: %MONGO_SVC%
  exit /b 1
)

kubectl -n %NAMESPACE% get deployment %SEED_CONTROLLER% >NUL 2>NUL
if not errorlevel 1 (
  echo Waiting for seed controller rollout...
  kubectl -n %NAMESPACE% rollout status deployment/%SEED_CONTROLLER% --timeout=120s >NUL
)

if /I "%APPLY_SEED%"=="true" (
  if exist "%SEED_MANIFEST%" (
    echo Applying seed manifest...
    kubectl apply -n %NAMESPACE% -f "%SEED_MANIFEST%" >NUL
    if errorlevel 1 (
      echo Failed to apply seed manifest.
    ) else (
      echo Waiting for TaskSeed "%SEED_NAME%" to finish...
      powershell -NoProfile -ExecutionPolicy Bypass -Command ^
        "$ns='%NAMESPACE%'; $name='%SEED_NAME%'; $timeout=%SEED_TIMEOUT_SEC%; $start=Get-Date; " ^
        "while((Get-Date) - $start -lt (New-TimeSpan -Seconds $timeout)) { " ^
        "  $phase = kubectl -n $ns get taskseed $name -o jsonpath='{.status.phase}' 2>$null; " ^
        "  if($phase -eq 'Succeeded'){ Write-Output 'TaskSeed succeeded'; exit 0 } " ^
        "  if($phase -eq 'Failed'){ Write-Output 'TaskSeed failed'; exit 1 } " ^
        "  Start-Sleep -Seconds 2 " ^
        "} " ^
        "Write-Output 'TaskSeed still pending'; exit 1"
    )
  ) else (
    echo Seed manifest not found: %SEED_MANIFEST%
  )
)

call :resolveport 8081 FRONTEND_PORT
call :resolveport 8080 API_PORT
call :resolveport 27017 MONGO_PORT

echo Starting port-forwards (new windows)...
start "pf-frontend" cmd /c kubectl -n %NAMESPACE% port-forward svc/%FRONTEND_SVC% %FRONTEND_PORT%:80
start "pf-api" cmd /c kubectl -n %NAMESPACE% port-forward svc/%API_SVC% %API_PORT%:8080
start "pf-mongodb" cmd /c kubectl -n %NAMESPACE% port-forward svc/%MONGO_SVC% %MONGO_PORT%:27017

echo Frontend: http://localhost:%FRONTEND_PORT%
echo API:      http://localhost:%API_PORT%
echo MongoDB:  mongodb://localhost:%MONGO_PORT%

popd

exit /b 0

:require
where %1 >NUL 2>NUL
if errorlevel 1 (
  echo Missing command: %1
  exit /b 1
)
exit /b 0

:resolveport
set "PORT=%~1"
set "VAR=%~2"
for /f %%p in ('powershell -NoProfile -Command "$p=%PORT%; $c=@(Get-NetTCPConnection -LocalPort $p -ErrorAction SilentlyContinue).Count; if($c -eq 0){$p}else{$n=$p+1; while(@(Get-NetTCPConnection -LocalPort $n -ErrorAction SilentlyContinue).Count -gt 0){$n++}; $n }"') do set "%VAR%=%%p"
call set "RES=%%%VAR%%%"
if not "%PORT%"=="%RES%" echo Port %PORT% in use. Using %RES% instead.
exit /b 0
