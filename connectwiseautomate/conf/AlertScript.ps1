# Script Parameters for CW: "@eu" "@apiKey@" "@Status@" "%clientname%" "%computername%" "%locationname%" "@Fieldname@" "@Result@" "%when%" "%ContactName%" "@Monitor@"

param (
  [string]$eu = "False",
  [string]$apiKey = "apiKey",
  [string]$status = "status",
  [string]$clientname = "client",
  [string]$computername = "computer",
  [string]$locationname = "location",
  [string]$fieldname = "field",
  [string]$result = "result",
  [string]$when = "when",
  [string]$contactName = "contact",
  [string]$monitorName = "monitor"
)

$uri = "https://api.atlassian.com/jsm/ops/integration/v1/json/integrations/webhooks/connectwiseautomate?apiKey=" + $apiKey;

$body = ConvertTo-Json @{
  status = $status
  clientName = $clientname
  computerName = $computername
  locationName = $locationname
  fieldName = $fieldname
  result = $result
  when = $when
  contactName = $contactName
  monitorName = $monitorName
}
$headers = [Hashtable] @{
  Type = 'application/json'
};
$result = Invoke-RestMethod -Method "Post" -Uri $uri -Headers $headers -Body $body -ContentType 'application/json'
Write-Output $result
