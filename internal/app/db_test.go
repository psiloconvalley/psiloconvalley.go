package app

import "testing"

func TestCheckProdGuard(t *testing.T) {
	tests := []struct {
		name      string
		dsn       string
		allowProd string
		want      ProdGuardVerdict
		wantMarker string
	}{
		{
			name:      "local postgres is allowed",
			dsn:       "postgresql://postgres:localdev@localhost:5432/psiloconvalley",
			allowProd: "",
			want:      GuardAllow,
			wantMarker: "",
		},
		{
			name:      "127.0.0.1 is allowed",
			dsn:       "postgresql://user:pw@127.0.0.1:5432/db",
			allowProd: "",
			want:      GuardAllow,
		},
		{
			name:      "railway proxy host is refused without ALLOW_PROD",
			dsn:       "postgresql://postgres:xxx@junction.proxy.rlwy.net:10442/railway",
			allowProd: "",
			want:      GuardRefuse,
			wantMarker: "rlwy.net",
		},
		{
			name:      "railway proxy host is allowed with ALLOW_PROD=1 but warns",
			dsn:       "postgresql://postgres:xxx@junction.proxy.rlwy.net:10442/railway",
			allowProd: "1",
			want:      GuardAllowWithWarning,
			wantMarker: "rlwy.net",
		},
		{
			name:      "railway.app host is refused without ALLOW_PROD",
			dsn:       "postgresql://user:pw@containers-us-west-1.railway.app:7777/db",
			allowProd: "",
			want:      GuardRefuse,
			wantMarker: "railway.app",
		},
		{
			name:      "internal railway host is refused",
			dsn:       "postgresql://user:pw@postgres.railway.internal:5432/db",
			allowProd: "",
			want:      GuardRefuse,
			wantMarker: ".railway.",
		},
		{
			name:      "uppercase RLWY.NET still matches",
			dsn:       "postgresql://user:pw@HOST.RLWY.NET:10442/db",
			allowProd: "",
			want:      GuardRefuse,
			wantMarker: "rlwy.net",
		},
		{
			name:      "ALLOW_PROD values other than 1 do not bypass",
			dsn:       "postgresql://user:pw@foo.rlwy.net:10442/db",
			allowProd: "true",
			want:      GuardRefuse,
			wantMarker: "rlwy.net",
		},
		{
			name:      "ALLOW_PROD=yes does not bypass",
			dsn:       "postgresql://user:pw@foo.rlwy.net:10442/db",
			allowProd: "yes",
			want:      GuardRefuse,
			wantMarker: "rlwy.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVerdict, gotMarker := CheckProdGuard(tt.dsn, tt.allowProd)
			if gotVerdict != tt.want {
				t.Errorf("verdict: got %v, want %v", gotVerdict, tt.want)
			}
			if tt.wantMarker != "" && gotMarker != tt.wantMarker {
				t.Errorf("marker: got %q, want %q", gotMarker, tt.wantMarker)
			}
		})
	}
}
