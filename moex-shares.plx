use strict;
use warnings;

use LWP::UserAgent;
use Encode qw(decode);
use JSON qw(from_json);

my $ua = LWP::UserAgent->new(timeout => 10);

my ($offset, %emitents) = (0, ());
while (1) {
	my $resp = $ua->get("https://iss.moex.com/iss/securities.json?engine=stock&market=shares&iss.meta=off&securities.columns=secid,isin,emitent_title,emitent_inn&start=$offset");
	die "request failed: " . $resp->status_line() . "\n"
		if not $resp->is_success();

	my $ref = eval { from_json($resp->decoded_content()) };
	die "can't decode json: $@\n"
		if not $ref;

	foreach my $row (@{ $ref->{securities}{data} }) {
		my ($sec_id, $isin, $emitent_name, $inn) = @$row;

		if (defined $inn) {
			my $ref = { sec_id => $sec_id, isin => $isin, emitent_name => $emitent_name, inn => $inn };
			$emitents{ sec_id }{ $sec_id } = $ref;
			$emitents{ isin }{ $isin } = $ref;
		}

		$offset++;
	}

	last if not scalar(@{ $ref->{securities}{data} });
}

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
		inn => $emitents{isin}{ $isin }{inn} // '<undef>',
	};
}
foreach my $row (@{ $ref->{marketdata}{data} }) {
	my ($sec_id, $capitalization) = @{ $row };

	$data{ $sec_id }{capitalization} = $capitalization // '0';
}

foreach my $sec_id (sort { $data{$b}{capitalization} <=> $data{$a}{capitalization} } keys %data) {
	my $row = $data{ $sec_id };

	print "$sec_id $row->{isin} $row->{inn} $row->{sec_name} $row->{short_name} cnt: $row->{share_cnt} cap: $row->{capitalization}\n";
}
