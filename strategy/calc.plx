#!/usr/bin/env perl

use strict;
use warnings;

use Data::Dumper;
use Getopt::Long;
use File::Basename;

my %args = (
	types => '',

	date_begin => '',
	date_end => '',

	initial_payment => 0.0,

	extra_payment => 0.0,
	extra_payment_period_mon => 0,
	extra_payment_rebalance => 0,

	rebalance_period_mon => 0,
	rebalance_min_percent => 1.0,
);

Getopt::Long::Configure('auto_help');
GetOptions(
	'types=s' => \$args{types},

	'date_begin=s' => \$args{date_begin},
	'date_end=s' => \$args{date_end},

	'initial_payment=f' => \$args{initial_payment},

	'extra_payment=f' => \$args{extra_payment},
	'extra_payment_period_mon=i' => \$args{extra_payment_period_mon},
	'extra_payment_rebalance' => \$args{extra_payment_rebalance},

	'rebalance_period_mon=i' => \$args{rebalance_period_mon},
	'rebalance_min_percent=f' => \$args{rebalance_min_percent},
) or die "Usage: $0 --date_begin 'yyyy-mm' --date_end 'yyyy-mm' --types=... <opts...>\n";

die "missed `--date_begin=yyyy-mm' or `--date_end=yyyy-mm' option\n"
	if not $args{date_begin} or not $args{date_end};

my ($yyyy_begin, $mm_begin) = parse_date($args{date_begin});
my ($yyyy_end, $mm_end) = parse_date($args{date_end});

my @types = split /,/, $args{types};
foreach my $type (@types) {
	die "unknown type `$type'"
		if not -f "data/$type.txt";
}
if (not @types) {
	@types = map { basename($_, '.txt') } glob 'data/*.txt';
}

my %cache;
my $min_common_date = { yyyy => $yyyy_begin, mm => $mm_begin };
my $max_common_date = { yyyy => $yyyy_end, mm => $mm_end };
foreach my $type (@types) {
	$cache{ $type } = read_data($type);

	if ($min_common_date->{yyyy} < $cache{$type}[0]{yyyy}) {
		$min_common_date = { yyyy => $cache{$type}[0]{yyyy}, mm => $cache{$type}[0]{mm} };
	} elsif ($min_common_date->{yyyy} == $cache{$type}[0]{yyyy} and $min_common_date->{mm} < $cache{$type}[0]{mm}) {
		$min_common_date = { yyyy => $cache{$type}[0]{yyyy}, mm => $cache{$type}[0]{mm} };
	}

	if ($max_common_date->{yyyy} > $cache{$type}[-1]{yyyy}) {
		$max_common_date = { yyyy => $cache{$type}[-1]{yyyy}, mm => $cache{$type}[-1]{mm} };
	} elsif ($max_common_date->{yyyy} == $cache{$type}[-1]{yyyy} and $max_common_date->{mm} > $cache{$type}[-1]{mm}) {
		$max_common_date = { yyyy => $cache{$type}[-1]{yyyy}, mm => $cache{$type}[-1]{mm} };
	}
}

# отфильтровываем данные вне запрошенного диапазона + приводим сужаем диапазон чтобы для всех типов были нужные данные
my $item_cnt;
foreach my $type (@types) {
	my $arr = $cache{ $type };

	my ($off, $min_off, $extra_cnt) = (0, 0, undef);
	foreach my $row (@{ $arr }) {
		$off++;

		if ($row->{yyyy} < $min_common_date->{yyyy} or
		    ($row->{yyyy} == $min_common_date->{yyyy} and $row->{mm} < $min_common_date->{mm})) {
			$min_off = $off;
		}

		if ($row->{yyyy} > $max_common_date->{yyyy} or
		    ($row->{yyyy} == $max_common_date->{yyyy} and $row->{mm} > $max_common_date->{mm})) {
			$extra_cnt //= scalar(@{ $arr }) - $off + 1;
		}
	}

	splice @{$arr}, 0, $min_off;
	if ($extra_cnt) {
		$#{ $arr } -= $extra_cnt;
	}

	my $cnt = scalar(@{ $arr });
	if (not defined $item_cnt) {
		$item_cnt = $cnt;
	} elsif ($cnt != $item_cnt) {
		die "expected $item_cnt items, but got $cnt items for type `$type'"
	}
}

my $result_date = "[$min_common_date->{yyyy}/$min_common_date->{mm} $max_common_date->{yyyy}/$max_common_date->{mm}]";
foreach my $type (@types) {
	if ($type eq 'inflation') {
		my $result = 1.0;
		for (my $i = 0; $i < scalar(@{ $cache{ $type } }); $i++) {
			$result *= $cache{ $type }[$i]{val} / 100.0;
		}
		$result = ($result - 1) * 100.0;

		printf "$result_date $type %.2f\n", $result;
	} else {
		die "type `$type' is not supported\n"
	}
}

sub read_data
{
	my $type = shift;

	my $path = "data/$type.txt";

	my @ret;
	open my $fh, '<', $path
		or die "can't open `$path': $!";
	while (my $line = <$fh>) {
		chomp $line;

		my ($date, $val) = split /\s+/, $line, 2;
		my ($yyyy, $mm) = parse_date($date);

		push @ret, { yyyy => $yyyy, mm => $mm, val => $val }
	}

	return \@ret;
}

sub parse_date
{
	my $date = shift;

	my ($yyyy, $mm) = split qr{/}, $date, 2;
	die "bad format `$date', expected `yyyy/mm value'"
		if not $yyyy or not $mm;

	return ($yyyy, $mm);
}
