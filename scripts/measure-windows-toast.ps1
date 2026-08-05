# Measures the latency of Fyne's Windows toast path: write a short PowerShell
# script to %TEMP% and run it via PowerShell -ExecutionPolicy Bypass, the same
# approach fyne.io/fyne/v2/app uses in app_windows.go SendNotification.
param(
    [int]$Iterations = 3
)

$template = @'
$title = "GoSentry timing test"
$content = "benchmark"
$iconPath = "file:///"
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastImageAndText02)
$toastXml = [xml] $template.GetXml()
$toastXml.GetElementsByTagName("text")[0].AppendChild($toastXml.CreateTextNode($title)) > $null
$toastXml.GetElementsByTagName("text")[1].AppendChild($toastXml.CreateTextNode($content)) > $null
$toastXml.GetElementsByTagName("image")[0].SetAttribute("src", $iconPath) > $null
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($toastXml.OuterXml)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("ru.mixeme.gosentry.desktop").Show($toast);
'@

Write-Host "Fyne-style Windows toast latency ($Iterations run(s), no icon path):"
$totalMs = 0
for ($i = 1; $i -le $Iterations; $i++) {
    $scriptPath = Join-Path $env:TEMP ("fyne-timing-test-$i.ps1")
    Set-Content -Path $scriptPath -Value $template -Encoding UTF8
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $launch = "(Get-Content -Encoding UTF8 -Path `"$scriptPath`" -Raw) | Invoke-Expression"
    & PowerShell -ExecutionPolicy Bypass -Command $launch | Out-Null
    $sw.Stop()
    $ms = [int]$sw.ElapsedMilliseconds
    $totalMs += $ms
    Write-Host ("  run {0}: {1} ms" -f $i, $ms)
    Remove-Item $scriptPath -ErrorAction SilentlyContinue
}
$avg = [math]::Round($totalMs / [double]$Iterations)
Write-Host ("  average: {0} ms" -f $avg)
