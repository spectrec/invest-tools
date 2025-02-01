use strict;
use warnings;

use LWP::UserAgent;
use Encode qw(decode);
use JSON qw(from_json);

my $ua = LWP::UserAgent->new(timeout => 30);

my ($offset, %emitents) = (0, ());
while (1) {
	my $resp;
	for (my $i = 0; $i < 3; $i++) {
		print {*STDERR} "fetch https://iss.moex.com/iss/securities.json off=$offset\n";

		$resp = $ua->get("https://iss.moex.com/iss/securities.json?engine=stock&market=shares&iss.meta=off&securities.columns=secid,isin,emitent_title,emitent_inn&start=$offset");
		last if $resp->is_success();

		warn "request https://iss.moex.com/iss/securities.json off=$offset failed: " . $resp->status_line() . "\n";
		$resp = undef;
	}
	next if not $resp;

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

my $resp;
for (my $i = 0; $i < 3; $i++) {
	print {*STDERR} "fetch https://iss.moex.com/iss/engines/stock/markets/shares/boards/TQBR/securities.json\n";

	$resp = $ua->get('https://iss.moex.com/iss/engines/stock/markets/shares/boards/TQBR/securities.json?marketdata.columns=SECID,ISSUECAPITALIZATION&securities.columns=SECID,SECNAME,ISIN,ISSUESIZE');
	last if $resp->is_success();

	warn "request https://iss.moex.com/iss/engines/stock/markets/shares/boards/TQBR/securities.json failed: " . $resp->status_line() . "\n";
	$resp = undef;
}
die "can't fetch securities list: no more tries\n"
	if not $resp;

my $ref = eval { from_json($resp->decoded_content()) };
die "can't decode json: $@\n"
	if not $ref;

my %data;
foreach my $row (@{ $ref->{securities}{data} }) {
	my ($sec_id, $sec_name, $isin, $share_cnt) = @{ $row };

	$data{ $sec_id } = {
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

my %skip;
foreach my $sec_id (sort { $data{$b}{capitalization} <=> $data{$a}{capitalization} } keys %data) {
	next if $skip{ $sec_id };

	my $row = $data{ $sec_id };

	my $pref_id = "${sec_id}P";
	if (my $pref = $data{ $pref_id }) {
		$skip{ $pref_id } = 1;

		$row->{capitalization} += $pref->{capitalization};
		$row->{share_cnt} += $pref->{share_cnt};
		$sec_id = "$sec_id,$pref_id";
	}

	printf "ticker: %- 12s isin: %- 14s inn: %- 12s cnt: % 18s  cap: % 18s  name: %s\n",
	         $sec_id,   $row->{isin},   $row->{inn}, num2str($row->{share_cnt}), num2str($row->{capitalization}), $row->{sec_name};
}

sub num2str
{
	my $num = shift;

	my @parts;
	while ($num) {
		unshift @parts, sprintf("%03d", $num % 1000);
		$num = int($num / 1000);
	}

	my $r = join '_', @parts;
	$r =~ s/^0+//;

	return $r;
}
