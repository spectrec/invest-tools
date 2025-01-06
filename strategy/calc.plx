#!/usr/bin/env perl

use strict;
use warnings;

use Data::Dumper;
use Getopt::Long;
use File::Basename;


my %simple_types = map { $_ => 1 } qw(arsagera_fa gldrub mcftr pldrub pltrub slvrub usdrub);


my %args = (
	debug => 0,

	types => '',

	date_begin => '',
	date_end => '',
	duration => '',

	initial_payment => 0.0,

	extra_payment => 0.0,
	extra_payment_period_mon => 0,

	rebalance_period_mon => 0,
);

Getopt::Long::Configure('auto_help');
GetOptions(
	'debug' => \$args{debug},

	'types=s' => \$args{types},

	'date_begin=s' => \$args{date_begin},
	'date_end=s' => \$args{date_end},
	'duration=s' => \$args{duration},

	'initial_payment=f' => \$args{initial_payment},

	'extra_payment=f' => \$args{extra_payment},
	'extra_payment_period_mon=i' => \$args{extra_payment_period_mon},

	'rebalance_period_mon=i' => \$args{rebalance_period_mon},
) or die "Usage: $0 --date_begin 'yyyy/mm' --date_end 'yyyy/mm' --types=... <opts...>\n";

die "missed `--date_begin=yyyy/mm' or `--date_end=yyyy/mm' option\n"
	if not $args{date_begin} or (not $args{date_end} and not $args{duration});

my @types = split /,/, $args{types};
if (not @types) {
	@types = map { basename($_, '.txt') } glob 'data/*.txt';
	die "missed `--types' option, possible values: [@types]\n"
}

my ($weight_sum, %plan) = (0);
foreach my $type (@types) {
	my ($name, $weight) = split /:/, $type, 2;

	$type = $name;
	die "unknown type `$type'"
		if not -f "data/$type.txt";

	if ($type ne 'inflation') {
		$plan{ $type } = $weight // 100;
		$weight_sum += $plan{ $type };
	}
}
die "bad total weight `$weight_sum' (must be 0 or 100)\n"
	if $weight_sum != 100;

my $max_date;
my $min_date = Date->parse($args{date_begin});
if ($args{date_end}) {
	$max_date = Date->parse($args{date_end});
} else {
	my ($yyyy, $mm) = $args{duration} =~ /(\d+)y(\d+)m/;
	die "bad `--duration' format, must be `<NUM>y<NUM>m'\n"
		if not defined $mm;

	if ($yyyy > 0 && $mm == 0) {
		# костыль, чтобы при добавлении 1y0m к 2000/01 получали 2000/12
		$yyyy--;
		$mm = 11;
	}

	$max_date = $min_date->copy()->add(yyyy => $yyyy, mm => $mm);
}

die "--date_begin must be lower than --date_end\n"
	if not $min_date->before($max_date);

my %cache;
foreach my $type (@types) {
	$cache{ $type } = read_data($type);

	if ($min_date->before($cache{$type}[0]{date})) {
		$min_date = $cache{$type}[0]{date}->copy();
	}
	if ($max_date->after($cache{$type}[-1]{date})) {
		$max_date = $cache{$type}[-1]{date}->copy();
	}
}

# отфильтровываем данные вне запрошенного диапазона + приводим сужаем диапазон чтобы для всех типов были нужные данные
my $item_cnt;
foreach my $type (@types) {
	my $arr = $cache{ $type };

	my ($off, $min_off, $extra_cnt) = (0, 0, undef);
	foreach my $row (@{ $arr }) {
		$off++;

		if ($row->{date}->before($min_date)) {
			$min_off = $off;
		}

		if ($row->{date}->after($max_date)) {
			$extra_cnt = scalar(@{ $arr }) - $off + 1;
			last;
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

my %portfolio = (
	inflation => 1.0,
	inflation_result => $args{initial_payment},

	in_sum => $args{initial_payment},
	cash => $args{initial_payment},

	assets => {},
);

my $date = $min_date->copy();
for (my $i = 0; ; $i++, $date->next()) {
	debug("%s start", $date->to_string());

	# делаю это до проверки текущей даты, чтобы последний месяц для бондов и депозитов тоже зачитывался
	foreach my $type (qw(deposits bonds)) {
		next if not $plan{ $type };

		# добавляем накопленный доход за месяц для вкладов и облигаций
		foreach my $sec (@{ $portfolio{assets}{ $type } }) {
			$sec->{yield_sum} += $sec->{yield_mm};
		}

		# экспирация вкладов и облигаций
		while ($portfolio{assets}{ $type }[0] and $date->equal($portfolio{assets}{ $type }[0]{expire_date})) {
			my $sec = shift @{ $portfolio{assets}{ $type } };
			$portfolio{cash} += $sec->{yield_sum};
			debug("expire $type, add value %.2f, cash = %.2f", $sec->{yield_sum}, $portfolio{cash});
		}
	}

	last if $date->after($max_date);

	if ($args{extra_payment_period_mon} and
	    (($i + 1) % $args{extra_payment_period_mon}) == 0) {
		$portfolio{inflation_result} += $args{extra_payment};
		$portfolio{in_sum} += $args{extra_payment};
		$portfolio{cash} += $args{extra_payment};
	}

	if ($cache{inflation}) {
		# инфляцию считаею как рекомендует сам цб в своей эксельке с данными
		$portfolio{inflation} *= $cache{inflation}[$i]{val} / 100.0;

		# имитация вклада с капитализацией и ежемесячными выплатами, проценты по которому равны инфляции в этом месяце
		$portfolio{inflation_result} *= $cache{inflation}[$i]{val} / 100.0;
	}

	if ($args{rebalance_period_mon} and
	    (($i + 1) % $args{rebalance_period_mon}) == 0) {
		debug('try rebalance');

		# тут просто продаю все активы для которых знаю текущие цены (т.е. все кроме облигаций и вкладов),
		#  на следующем этеапе они будут куплены в соответствии с требуемой пропорцией
		foreach my $type (@types) {
			next if not $simple_types{ $type };

			my $cnt = $portfolio{assets}{$type}{cnt};
			$portfolio{assets}{ $type }{cnt} = 0;

			$portfolio{cash} += $cache{ $type }[$i]{val} * $cnt;
		}
	}

	# вычисляем денежный эквивалент портфеля, чтобы прикинуть сколько денег
	#  нужно потратить на покупку активов каждого типа
	my %value_by_type;
	my $portfolio_value = $portfolio{cash};
	foreach my $type (keys %{ $portfolio{assets} }) {
		if ($simple_types{ $type }) {
			$value_by_type{ $type } = $portfolio{assets}{ $type }{cnt} * $cache{ $type }[$i]{val};
		} elsif ($type eq 'deposits' or $type eq 'bonds') {
			$value_by_type{ $type } = 0;
			foreach my $sec (@{ $portfolio{assets}{ $type } }) {
				$value_by_type{ $type } += $sec->{yield_sum};
			}
		} else {
			die "unknown type `$type'"
		}

		$portfolio_value += $value_by_type{ $type };
	}

	my %need_skip;
	my $overflow_sum = 0.0;
	foreach my $type (keys %plan) {
		my $planned = $portfolio_value * $plan{ $type } / 100.0;
		my $actual = $value_by_type{ $type } // 0;

		# активов заданного типа больше чем должно быть, но продать полностью или частично мы их сейчас не можем, т.к.
		# - либо это вклад/облигация (предполагаем что их всегда держим до погашения)
		# - либо ещё не настало время ребалансировки
		#
		# поэтому просто равномерно вычту сумму превышения из денег, на которые будем докупать остальные активы,
		#  + убираю из плана покупок активы которых уже достаточно
		if ($actual > $planned) {
			$overflow_sum += $actual - $planned;
			$need_skip{ $type } = 1;
		}
	}
	if ($overflow_sum > 0) {
		$portfolio_value -= $overflow_sum;
	}
	debug('current portolio value excluding overflow part (%.2f): %.2f', $overflow_sum, $portfolio_value);

	# обходим все актиы, даже если их вес больше запланированного,
	#  чтобы обновить текущую оценку (`value') для простых активов
	foreach my $type (keys %plan) {
		my $planned = $portfolio_value * $plan{ $type } / 100.0;
		my $actual = $value_by_type{ $type } // 0;

		my $cash = $planned - $actual;
		if ($need_skip{ $type }) {
			$cash = 0;
		} else {
			debug("$type: available cash %.2f", $cash);
		}

		if ($simple_types{ $type }) {
			my $price = $cache{ $type }[$i]{val};

			my $cnt = int($cash / $price);
			if ($cnt > 0) {
				$portfolio{assets}{ $type }{cnt} += $cnt;
				debug("$type: buy $cnt items");
			}
			$portfolio{assets}{ $type }{value} = $portfolio{assets}{ $type }{cnt}*$price;

			my $spent = $cnt*$price;
			reduce_cash($spent);

			# сдачу добавляем обратно к оценке, чтобы на нее можно было докупить другие активы
			$portfolio_value += $cash - $spent;
		} elsif ($type eq 'deposits' or $type eq 'bonds') {
			next if $cash < 1000; # не открываем депозит если у нас меньше 1к денег

			# используем годовые депозиты/облигации
			my $yield_mm = $cash * $cache{ $type }[$i]{val}/100.0 / 12;
			push @{ $portfolio{assets}{ $type } }, {
				expire_date => $date->copy()->add(mm => 12),

				yield_sum => $cash,
				yield_mm => $yield_mm,

				percent => $cache{ $type }[$i]{val},
			};

			reduce_cash($cash);
		}
	}

	debug("%s end, state: %s", $date->to_string(), dump_portfolio($date->diff_years($min_date)));
}

printf "[%s .. %s] result: %s\n", $min_date->to_string(), $max_date->to_string(), dump_portfolio($date->diff_years($min_date));
exit 0;


sub dump_portfolio
{
	my ($spent_years) = @_;

	my $portfolio_value = $portfolio{cash};
	foreach my $type (keys %{ $portfolio{assets} }) {
		if (ref($portfolio{assets}{ $type }) eq 'HASH') {
			$portfolio_value += $portfolio{assets}{ $type }{value};
		} else {
			foreach my $sec (@{ $portfolio{assets}{ $type } }) {
				$portfolio_value += $sec->{yield_sum};
			}
		}
	}

	my $result = sprintf 'in_sum: %.2f result_sum: %.2f yield: %.2f%%', $portfolio{in_sum}, $portfolio_value,
			     calc_yield($portfolio{in_sum}, $portfolio_value, $spent_years);
	if ($portfolio{inflation}) {
		$result .= sprintf ' inflation_result: %.2f inflation_yield: %.2f', $portfolio{inflation_result},
			           calc_yield($portfolio{in_sum}, $portfolio{inflation_result}, $spent_years);
	}

	if (keys %{ $portfolio{assets} }) {
		$result .= ' assets: [';
	}

	foreach my $type (keys %{ $portfolio{assets} }) {
		if (ref($portfolio{assets}{ $type }) eq 'HASH') {
			$result .= sprintf " $type:{value: %.2f, cnt=%u}", $portfolio{assets}{ $type }{value}, ($portfolio{assets}{ $type }{cnt} // 0);
		} else {
			$result .= "$type: [";
			foreach my $sec (@{ $portfolio{assets}{ $type } }) {
				$result .= sprintf " {value: %.2f, pct: %.2f, exp: %s}", $sec->{yield_sum}, $sec->{percent}, $sec->{expire_date}->to_string();
			}
			$result .= " ]";
		}
	}

	if (keys %{ $portfolio{assets} }) {
		$result .= ' ]';
	}

	return $result;
}

sub calc_yield
{
	my ($in, $result, $spent_years) = @_;

	return 0.0 if not $result;

	# r[1] = in * (1 + pct)
	# r[2] = in * (1 + pct) * (1 + pct) = in * (1+pct)**2
	# r[3] = in * (1+pct)**3
	# ...
	# r[n] = in * (1+pct)**n
	# => r[n]/v = (1+pct)**n
	# => (r[n]/v)**(1/n) = 1 + pct
	# => pct = (r[n]/v)**(1/n) - 1
	return (($result/$in) ** (1.0/$spent_years) - 1) * 100.0
}

sub reduce_cash
{
	my $value = shift;

	if ($portfolio{cash} < $value) {
		my $diff = $value - $portfolio{cash};

		die "portfolio cash underflow ($portfolio{cash} < $value)"
			if $diff > 0.001;
	}

	$portfolio{cash} -= $value;
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
		push @ret, { date => Date->parse($date), val => $val }
	}

	return \@ret;
}

sub debug
{
	return if not $args{debug};

	my $fmt = shift;
	printf "$fmt\n", @_
}


package Date;

sub new
{
	my ($class, $yyyy, $mm) = @_;

	return bless { _yyyy => $yyyy, _mm => $mm }, $class;
}

sub copy
{
	my ($self) = @_;

	return __PACKAGE__->new($self->{_yyyy}, $self->{_mm});
}

sub parse
{
	my ($class, $date) = @_;

	my ($yyyy, $mm) = split qr{/}, $date, 2;
	die "bad format `$date', expected `yyyy/mm value'"
		if not $yyyy or not $mm;

	return $class->new($yyyy, $mm);
}

sub before
{
	my ($self, $date) = @_;

	return 0 if $self->{_yyyy} > $date->{_yyyy};
	return 1 if $self->{_yyyy} < $date->{_yyyy};

	return $self->{_mm} < $date->{_mm};
}

sub after
{
	my ($self, $date) = @_;

	return 0 if $self->{_yyyy} < $date->{_yyyy};
	return 1 if $self->{_yyyy} > $date->{_yyyy};

	return $self->{_mm} > $date->{_mm};
}

sub equal
{
	my ($self, $date) = @_;

	return $self->{_yyyy} == $date->{_yyyy} && $self->{_mm} == $date->{_mm};
}

sub add
{
	my $self = shift;

	my %args = (yyyy => 0, mm => 0, @_);
	$self->{_yyyy} += $args{yyyy};

	for (my $i = 0; $i < $args{mm}; $i++) {
		if ($self->{_mm} == 12) {
			$self->{_yyyy}++;
			$self->{_mm} = 0;
		}

		$self->{_mm}++;
	}

	return $self;
}

sub next
{
	my $self = shift;

	return $self->add(mm => 1);
}

sub diff_years
{
	my ($self, $date) = @_;

	my $yyyy_diff = $self->{_yyyy} - $date->{_yyyy};
	my $mm_diff = (12 - $self->{_mm}) + $date->{_mm};

	return $yyyy_diff + $mm_diff / 12.0;
}

sub to_string
{
	my $self = shift;

	return "$self->{_yyyy}/$self->{_mm}"
}
