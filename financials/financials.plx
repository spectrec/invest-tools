#!/usr/bin/env perl

use strict;
use warnings;

use utf8;
use Encode;
use Cwd qw(abs_path);

my $COLUMN_KEY_LEN = 40;
my $COLUMN_VAL_LEN = 20;

binmode(STDOUT, "encoding(UTF-8)");

my ($config_path, $company_path) = @ARGV;
die "Usage: $0 <config> <company>\n"
	if not $company_path;

my $config_ref = eval { do(abs_path($config_path)) };
die "can't parse config `$config_path': $@\n"
	if $@;

my $company_ref = eval { do(abs_path($company_path)) };
die "can't parse company `$company_path': $@\n"
	if $@;

# init missing data
my $cnt = @{ $company_ref->{years} };

$company_ref->{'l/a'} = [(0) x $cnt];
$company_ref->{'op_income/interest'} = [(0) x $cnt];

$company_ref->{roe} = [(0) x $cnt];
$company_ref->{roe_avg} = 0;

$company_ref->{ros} = [(0) x $cnt];
$company_ref->{ros_avg} = 0;

$company_ref->{'fcf'} = [(0) x $cnt];
$company_ref->{'dps'} = [(0) x $cnt];
$company_ref->{'div/net_income'} = [(0) x $cnt];

$company_ref->{'eps'} = [(0) x $cnt];

$company_ref->{adj_avg_eps} = 0;
for (my $i = 0; $i < @{ $company_ref->{years} }; $i++) {
	$company_ref->{balance}{equity}[$i] = $company_ref->{balance}{assets}[$i] - $company_ref->{balance}{liabilities}[$i];
	$company_ref->{balance}{equity_per_share}[$i] = $company_ref->{balance}{equity}[$i] / $company_ref->{stock_count}[$i];

	$company_ref->{income}{adj_net_income}[$i] //= $company_ref->{income}{net_income}[$i];

	$company_ref->{'l/a'}[$i] = $company_ref->{balance}{liabilities}[$i] / $company_ref->{balance}{assets}[$i];

	if ($company_ref->{income}{operating_income}[$i] != 0 and $company_ref->{income}{interest_expences}[$i] != 0) {
		$company_ref->{'op_income/interest'}[$i] = $company_ref->{income}{operating_income}[$i] / $company_ref->{income}{interest_expences}[$i];
	} else {
		$company_ref->{'op_income/interest'} = undef;
	}

	$company_ref->{ros}[$i] = $company_ref->{income}{adj_net_income}[$i] / $company_ref->{income}{revenue}[$i] * 100;
	$company_ref->{ros_avg} += $company_ref->{ros}[$i];

	if ($i > 0) {
		$company_ref->{roe}[$i] = $company_ref->{income}{adj_net_income}[$i] / $company_ref->{balance}{equity}[$i - 1] * 100;
		$company_ref->{roe_avg} += $company_ref->{roe}[$i];

		$company_ref->{'div/net_income'}[$i] = $company_ref->{cache_flow}{dividends}[$i] / $company_ref->{income}{adj_net_income}[$i - 1] * 100;
	}

	if ($company_ref->{stock_count}[$i] != 0) {
		$company_ref->{'eps'}[$i] = $company_ref->{income}{adj_net_income}[$i] / $company_ref->{stock_count}[$i];
		$company_ref->{'dps'}[$i] = $company_ref->{cache_flow}{dividends}[$i] / $company_ref->{stock_count}[$i];

		$company_ref->{adj_avg_eps} += $company_ref->{'eps'}[$i];
	}

	$company_ref->{fcf}[$i] = $company_ref->{cache_flow}{net_operation_cf}[$i] + $company_ref->{cache_flow}{net_investing_cf}[$i];
}
$company_ref->{roe_avg} /= @{ $company_ref->{years} } - 1;
$company_ref->{ros_avg} /= @{ $company_ref->{years} } - 1;
$company_ref->{adj_avg_eps} /= @{ $company_ref->{years} } - 1;

my $hline = print_row('Год', $company_ref->{years});
print "$hline\n";
print_row('Активы', $company_ref->{balance}{assets}, \&num2str);
print_row('Обязательства', $company_ref->{balance}{liabilities}, \&num2str);
print_row('Капитал', $company_ref->{balance}{equity}, \&num2str);
print "$hline\n";

print "\n$hline\n";
print_row('Выручка', $company_ref->{income}{revenue}, \&num2str);
if ($company_ref->{income}{operating_income}[0]) {
	print_row('Операционная прибыль', $company_ref->{income}{operating_income}, \&num2str);
	print_row('Процентные расходы', $company_ref->{income}{interest_expences}, \&num2str);
}
print_row('Скорр ЧП', $company_ref->{income}{adj_net_income}, \&num2str);
print "$hline\n";

print "\n$hline\n";
print_row('FCF', $company_ref->{fcf}, \&num2str);
print_row('Дивиденды', $company_ref->{cache_flow}{dividends}, \&num2str);
print "$hline\n";

print "\n$hline\n";
print_row('Капитал на акцию', $company_ref->{balance}{equity_per_share}, \&num2str);
print_row('Дивиденды на акцию', $company_ref->{dps}, \&num2str);
print_row('EPS', $company_ref->{eps}, \&num2str);
print "$hline\n";

print_row('Обязательства/активы', $company_ref->{'l/a'}, \&num2str);
if ($company_ref->{income}{operating_income}[0]) {
	print_row('Оп. прибыль/проценты к уплате', $company_ref->{'op_income/interest'}, \&num2str);
}
print_row('Доля дивидендов в ЧП (%)', $company_ref->{'div/net_income'}, \&num2str);
print "$hline\n";

print_row('ROE', $company_ref->{roe}, \&num2str);
print_row('ROS', $company_ref->{ros}, \&num2str);
print "$hline\n";

print "\n$hline\n";
my $next_net_income_roe = $company_ref->{roe_avg} / 100 * $company_ref->{balance}{equity}[-1];
my $next_eps_roe = $next_net_income_roe/$company_ref->{stock_count}[-1];
printf("Средний ROE: %6.2f%%\n\tпрогноз ЧП: %s, прогноз EPS: %s\n\tсправедливая цена (%.1f%%): %s\n\tмаксимально-допустимая цена (%.1f%%): %s\n",
	$company_ref->{roe_avg},
	num2str($next_net_income_roe),
	num2str($next_eps_roe),
	$config_ref->{expected_return},
	num2str($next_eps_roe / $config_ref->{expected_return} * 100),
	$config_ref->{minimal_return},
	num2str($next_eps_roe / $config_ref->{minimal_return} * 100),
);

my $next_net_income_ros = $company_ref->{ros_avg} / 100 * $company_ref->{income}{revenue}[-1];
my $next_eps_ros = $next_net_income_ros/$company_ref->{stock_count}[-1];
printf("Средний ROS: %6.2f%%\n\tпрогноз ЧП: %s, прогноз EPS: %s\n\tсправедливая цена (%.1f%%): %s\n\tмаксимально-допустимая цена (%.1f%%): %s\n",
	$company_ref->{ros_avg},
	num2str($next_net_income_ros),
	num2str($next_eps_ros),
	$config_ref->{expected_return},
	num2str($next_eps_ros / $config_ref->{expected_return} * 100),
	$config_ref->{minimal_return},
	num2str($next_eps_ros / $config_ref->{minimal_return} * 100),
);

printf("Средняя EPS: %s\n\tсправедливая цена (%.1f%%): %s\n\tмаксимально-допустимая цена (%.1f%%): %s\n",
	num2str($company_ref->{adj_avg_eps}),
	$config_ref->{expected_return},
	num2str($company_ref->{adj_avg_eps} / $config_ref->{expected_return} * 100),
	$config_ref->{minimal_return},
	num2str($company_ref->{adj_avg_eps} / $config_ref->{minimal_return} * 100),
);

sub print_row
{
	my ($name, $array_ref, $format_cb) = @_;

	my $string = sprintf "|%- ${COLUMN_KEY_LEN}s|", $name;
	foreach my $row (@$array_ref) {
		my $v = $format_cb ? $format_cb->($row) : "$row";
		$string .= sprintf "% ${COLUMN_VAL_LEN}s|", $v;
	}
	print "$string\n";

	my $bytes_len = length($string);
	my $hline = "-" x $bytes_len;

	return $hline;
}

sub num2str
{
	my ($num) = @_;

	my $sign = '';
	if ($num < 0) {
		$sign = '-';
		$num = -$num;
	}

	return $sign . sprintf('%.3f', $num)
		if $num < 1000;

	return $sign . sprintf('%.3f тыс', $num / 1e3)
		if $num < 1e6;

	return $sign . sprintf('%.3f млн', $num / 1e6)
		if $num < 1e9;

	return $sign . sprintf('%.3f млрд', $num / 1e9)
		if $num < 1e12;

	return $sign . sprintf('%.3f трл', $num / 1e12)
}
