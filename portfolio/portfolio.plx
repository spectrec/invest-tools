#!/usr/bin/env perl

use strict;
use warnings;

use JSON;
use Data::Dumper;
use Getopt::Long;
use LWP::UserAgent;
use POSIX qw(abs mktime);

my %good_boards = map { $_ => 1 } qw(TQIF TQCB TQBR TQOB TQTF);

my $price_cache_path = 'price.cache';
GetOptions(
	"price-cache=s" => \$price_cache_path,
) or die usage();

my $input = shift
	or die usage();

my $input_ref = do {
	open my $fh, '<', $input
		or die "can't open `$input': $!\n";
	local $/;

	from_json(<$fh>)
};

my $sec_price_cache = do {
	my $ret;

	if (open my $fh, '<', $price_cache_path) {
		print "Use prices from cache: `$price_cache_path'\n";
		local $/;
		$ret = from_json(<$fh>)
	} elsif (not $!{ENOENT}) {
		die "can't open `$price_cache_path': $!";
	} else {
		$ret = {};
	}

	$ret;
};

my ($stat, $need_update_price_cache, @expired) = ({}, 0);
foreach my $part (@{ $input_ref->{portfolio} }) {
	foreach my $ref (@{ $part->{assets} }) {
		if (sec_is_expired($ref)) {
			push @expired, $ref->{isin};
		}

		if (not $ref->{price}) {
			if (not $sec_price_cache->{ $ref->{isin} }) {
				print "fetching price for `$ref->{isin}'\n";
				fetch_price($ref);

				$sec_price_cache->{ $ref->{isin} } = $ref->{price};
				$need_update_price_cache = 1;
			} else {
				$ref->{price} = $sec_price_cache->{ $ref->{isin} };
			}
		}

		my $price = full_price($ref);
		$stat->{price_by_type}{ $ref->{type} } += $price;
		$stat->{total_price} += $price;

		my $expected_cash_flow = expected_cash_flow($ref);
		$stat->{expected_cash_flow} += $expected_cash_flow;
		$stat->{cash_flow_by_type}{ $ref->{type} } += $expected_cash_flow;
	}
}

if ($need_update_price_cache) {
	my $tmp_path = "$price_cache_path.tmp";

	open my $fh, '>', $tmp_path
		or die "can't open `$tmp_path': $!\n";
	print {$fh} to_json($sec_price_cache)
		or die "can't write to `$tmp_path': $!\n";;
	close $fh
		or die "can't close `$tmp_path': $!\n";

	rename $tmp_path, $price_cache_path
		or die "can't rename `$tmp_path' -> `$price_cache_path': $!\n";
}

printf "\nTotal price: %s\n", price2str($stat->{total_price});
foreach my $type (sort { $stat->{price_by_type}{$b} <=> $stat->{price_by_type}{$a} } keys %{ $stat->{price_by_type} }) {
	my $percent = $stat->{price_by_type}{$type} / $stat->{total_price} * 100;

	my $plan = '';
	if ($input_ref->{asset_weight_plan}{ $type }) {
		my $expected = $stat->{total_price} / 100 * $input_ref->{asset_weight_plan}{ $type };
		my $diff = abs($stat->{price_by_type}{$type} - $expected);

		$plan = sprintf ", planned: %s (diff: %s, percent: %.1f%%)",
			price2str($expected), price2str($diff), $diff/$expected*100;
	}

	printf "\t%-15s: %s (%.1f%%)%s\n",
		$type, price2str($stat->{price_by_type}{$type}), $percent, $plan;
}

printf "\nExpected cash flow: %s (yield: %.2f%%), monthly: %s\n",
	price2str($stat->{expected_cash_flow}),
	$stat->{expected_cash_flow} / $stat->{total_price} * 100,
	price2str($stat->{expected_cash_flow} / 12);
foreach my $type (sort { $stat->{cash_flow_by_type}{$b} <=> $stat->{cash_flow_by_type}{$a} } keys %{ $stat->{cash_flow_by_type} }) {
	my $portfolio_yield_part = $stat->{cash_flow_by_type}{$type} / $stat->{expected_cash_flow} * 100;
	my $net_asset_yield_percent = $stat->{cash_flow_by_type}{$type} / $stat->{price_by_type}{$type} * 100;
	my $dirty_asset_yield_percent = $net_asset_yield_percent / 0.87;
	next if $stat->{cash_flow_by_type}{$type} == 0;

	printf "\t%-15s: %s, monthly: %s (cache flow part: %.1f%%, dirty asset yield: %.1f%%, net asset yield: %.1f%%)\n",
		$type, price2str($stat->{cash_flow_by_type}{$type}), price2str($stat->{cash_flow_by_type}{$type} / 12),
		$portfolio_yield_part, $dirty_asset_yield_percent, $net_asset_yield_percent;
}

if (@expired) {
	my %uniq = map { $_ => 1 } @expired;
	@expired = keys %uniq;

	print "WARNING: expired [@expired]\n";
}

sub usage
{
	return <<"EOF";
Usage: $0 [<opts>] <portfolio>

Options:
  --price-cache=<PATH>   use prices from <PATH> (default: $price_cache_path)
EOF
}

sub sec_is_expired
{
	my $ref = shift;

	return 0 if $ref->{type} ne 'bond';

	my ($year, $month, $day) = split /-/, $ref->{maturity_date};
	my $maturity_ts = mktime(0, 0, 0, $day, $month - 1, $year - 1900);

	return $maturity_ts < time();
}

sub price2str
{
	my $value = shift;

	my @ret;

	my $v = int($value / 1e6);
	if ($v > 0) {
		push @ret, "$v млн";
	}

	$v = int(($value % 1e6) / 1e3);
	if ($v > 0) {
		push @ret, "$v тыс";
	}

	return '0'
		if not @ret;

	return join ' ', @ret;
}

sub expected_cash_flow
{
	my ($sec) = @_;

	my $tax = 0.13;
	if ($sec->{type} eq 'bond') {
		return $sec->{nominal} * $sec->{percent} / 100.0 * $sec->{count} * (1 - $tax);
	}
	if ($sec->{type} eq 'div_fund') {
		return $sec->{raw_dividend} * $sec->{count} * $sec->{dividend_periods} * (1 - $tax);
	}
	if ($sec->{type} eq 'crowd_landing') {
		return $sec->{dividend} * $sec->{dividend_periods};
	}
	if ($sec->{type} eq 'stock' or $sec->{type} eq 'etf') {
		return $sec->{dividend_yield} / 100 * $sec->{price} * $sec->{lot_count} * (1 - $tax)
			if $sec->{dividend_yield};

		return 0;
	}

	return 0;
}

sub full_price
{
	my ($sec) = @_;

	if ($sec->{type} eq 'bond') {
		return $sec->{nominal} * $sec->{price} / 100.0 * $sec->{count};
	}
	if ($sec->{type} eq 'stock' or $sec->{type} eq 'etf') {
		return $sec->{price} * $sec->{lot_count} * $sec->{lot_size};
	}
	if ($sec->{type} eq 'fund' or $sec->{type} eq 'div_fund') {
		return $sec->{price} * $sec->{count};
	}

	return $sec->{price};
}

# https://iss.moex.com/iss/securities/RU000A104KU3.json?iss.meta=off - где искать параметры по ценной бумаге
sub fetch_price
{
	my ($security) = @_;

	my $url;
	if ($security->{type} eq 'bond') {
		$url = "http://iss.moex.com/iss/engines/stock/markets/bonds/securities/$security->{isin}.json?iss.meta=off&securities.columns=SECID,BOARDID,SHORTNAME,PREVPRICE";
	} elsif ($security->{type} eq 'stock' or $security->{type} eq 'etf') {
		$url = "http://iss.moex.com/iss/engines/stock/markets/shares/securities/$security->{ticker}.json?iss.meta=off&securities.columns=SECID,BOARDID,SHORTNAME,PREVPRICE";
	} else {
		$url = "http://iss.moex.com/iss/engines/stock/markets/shares/securities/$security->{isin}.json?iss.meta=off&securities.columns=SECID,BOARDID,SHORTNAME,PREVPRICE";
	}

	my $response = LWP::UserAgent->new(timeout => 10)->get($url);
	if (not $response->is_success()) {
		my ($code, $resp) = ($response->code(), $response->decoded_content());

		chomp $resp;
		die "request '$url' failed: $resp ($code)\n";
	}

	my $data = $response->decoded_content();

	my $ref = from_json($data);
	foreach my $v (@{ $ref->{securities}{data} }) {
		my ($sec_id, $board_id, $short_name, $price) = @$v;
		next if not $good_boards{ $board_id };

		$security->{price} = $price;
		return;
	}

	print Dumper $security, $ref;
	die "can't detect security price ($url)\n";
}
