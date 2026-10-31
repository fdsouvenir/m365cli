package auth
import("context";"testing";"time";"net/http";"io";"strings";"github.com/fdsouvenir/m365cli/internal/secrets")
func TestReviewChangedClientUsesOldCache(t *testing.T){
setTempSecretsHome(t); m:=NewManager()
if err:=m.saveToken("work",TokenCache{AccessToken:"old-client-token",Scope:"Mail.Read",ExpiresAt:time.Now().Add(time.Hour),IDToken:idTokenFor(t,"acct","tenant")});err!=nil{t.Fatal(err)}
token,err:=m.AcquireDelegatedToken(context.Background(),"work","DIFFERENT-CLIENT","DIFFERENT-AUTHORITY",[]string{"Mail.Read"},"acct","tenant")
if err!=nil{t.Fatal(err)};if token=="old-client-token"{t.Fatal("REPRODUCED: changed client/authority receives old token")}}
func TestReviewReadWriteCoversRead(t *testing.T){if !scopeStringCoversRequiredScopes("Mail.ReadWrite Files.ReadWrite",[]string{"Mail.Read","Files.Read"}){t.Fatal("REPRODUCED: broader read/write permissions rejected for reads")}}
func TestReviewFailedAppLoginPreservesSecret(t *testing.T){
setTempSecretsHome(t);m:=NewManager();key,_:=appSecretKey("work-app");if err:=secrets.SetSecret(key,[]byte("working-secret"));err!=nil{t.Fatal(err)}
m.HTTPClient=&http.Client{Transport:roundTripFunc(func(*http.Request)(*http.Response,error){return &http.Response{StatusCode:401,Header:make(http.Header),Body:io.NopCloser(strings.NewReader(`{"error":"invalid_client"}`))},nil})}
err:=m.LoginAppOnly(context.Background(),AppOnlyLoginInput{ProfileName:"work-app",ClientID:"client",Authority:"organizations",Secret:"bad-secret"});if err==nil{t.Fatal("expected rejected login")};got,err:=secrets.GetSecret(key);if err!=nil{t.Fatal(err)};if string(got)!="working-secret"{t.Fatal("REPRODUCED: rejected login replaced working saved secret")}}
