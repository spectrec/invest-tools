use strict;
use warnings;

use LWP::UserAgent;
use Encode qw(decode);
use JSON qw(from_json);

my $ua = LWP::UserAgent->new(timeout => 10);
my $resp = $ua->get('https://iss.moex.com/iss/engines/stock/markets/shares/boards/TQBR/securities.json?marketdata.columns=SECID,ISSUECAPITALIZATION&securities.columns=SECID,SHORTNAME,SECNAME,ISIN,ISSUESIZE');
die "request failed: " . $resp->status_line() . "\n"
	if not $resp->is_success();

my $ref = eval { from_json($resp->decoded_content()) };
die "can't decode json: $@\n"
	if not $ref;

my %data;
foreach my $row (@{ $ref->{securities}{data} }) {
	my ($sec_id, $short_name, $sec_name, $isin, $share_cnt) = @{ $row };

	$data{ $sec_id } = {
		short_name => $short_name,
		sec_name => $sec_name,
		isin => $isin,
		share_cnt => $share_cnt,
	};
}
foreach my $row (@{ $ref->{marketdata}{data} }) {
	my ($sec_id, $capitalization) = @{ $row };

	$data{ $sec_id }{capitalization} = $capitalization // '0';
}

foreach my $sec_id (sort { $data{$b}{capitalization} <=> $data{$a}{capitalization} } keys %data) {
	my $row = $data{ $sec_id };

	print "$sec_id $row->{isin} $row->{sec_name} $row->{short_name} cnt: $row->{share_cnt} cap: $row->{capitalization}\n";
}
