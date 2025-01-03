#!/usr/bin/env perl

use strict;
use warnings;

use JSON;
use LWP::UserAgent;

# Для расчета инфляции берем данные отсюда: https://rosstat.gov.ru/statistics/price (Индексы потребительских цен на товары и услуги по Российской Федерации, месяцы (с 1991 г.)).
# Из них достаем содержимое таблички с данными по инфляции, сохраняем в data/inflation.in и отдаем скрипту, он сохранит все в data/inflation.txt без каких-либо преобразований
# (только развернет таблицу в список)
{
	my $in = 'data/inflation.in';
	my $out = 'data/inflation.txt';

	open my $fh, '<', $in
		or die "can't open `$in': $!";

	my $data = [];
	while (my $line = <$fh>) {
		chomp $line;

		push @{ $data }, [ split /\s+/, $line ];
	}

	# Ожидаемый формат (должны быть только данные, без названий строк|столбцов):
	# месяц/год 1991 1992 1993 ...
	# январь     ...  ...
	# февраль    ...  ...
	# ...
	open my $os, '>', $out
		or die "can't open `$out': $!";
	foreach (my $year_idx = 0; ; $year_idx++) {
		last if not defined $data->[0][$year_idx];

		my $year = $year_idx + 1991;
		for (my $month_idx = 0; $month_idx < 12; $month_idx++) {
			my $val = $data->[$month_idx][$year_idx];
			last if not defined $val;

			my $month = $month_idx + 1;
			print {$os} "$year/$month $val\n";
		}
	}
	close $os;

	print "data from `$in' processed and stored into `$out'\n";
}


# Достаем отсюда xlsx файл: https://cbr.ru/currency_base/dynamics/?UniDbQuery.Posted=True&UniDbQuery.so=1&UniDbQuery.mode=1&UniDbQuery.date_req1=&UniDbQuery.date_req2=&UniDbQuery.VAL_NM_RQ=R01235&UniDbQuery.From=01.07.1992&UniDbQuery.To=29.12.2024
# Сохраняем данные из полученного xlsx в формате csv и отдаем скрипту, он посчитает среднее значение курса для каждого месяца и сохранит результат в data/usdrub.txt
{
	my $in = 'data/usdrub.in.csv';
	my $out = 'data/usdrub.txt';

	open my $fh, '<', $in
		or die "can't open `$in': $!";

	my $data = {};
	while (my $line = <$fh>) {
		chomp $line;

		my ($mm, $dd, $yyyy, $val) = $line =~ m{, (\d+)/(\d+)/(\d+) , (.+?) , Доллар \s+ США}msx;
		next if not $val;

		$val =~ s/"//g;
		$val =~ s/,//; # 1,000.0000 -> 1000.000

		if ($yyyy < 1998) {
			# в 1998 была деноминация, поэтому приводим цены до 98го к общей базе
			$val /= 1000.0;
		}

		push @{ $data->{$yyyy}{$mm} }, $val;
	}

	open my $os, '>', $out;
	foreach my $year (sort { $a <=> $b } keys %{ $data }) {
		foreach my $mm (sort { $a <=> $b } keys %{ $data->{$year} }) {
			my $items = $data->{$year}{$mm};

			my $sum = 0;
			foreach my $val (@{ $items }) {
				$sum += $val;
			}

			my $avg = $sum / scalar(@{ $items });
			printf {$os} "$year/$mm %.2f\n", $avg;
		}
	}
	close $os;

	print "data from `$in' processed and stored into `$out'\n";
}

my %month2num = (
	'января'	=> 1,
	'февраля'	=> 2,
	'марта'		=> 3,
	'апреля'	=> 4,
	'мая'		=> 5,
	'июня'		=> 6,
	'июля'		=> 7,
	'августа'	=> 8,
	'сентября'	=> 9,
	'октября'	=> 10,
	'ноября'	=> 11,
	'декабря'	=> 12,
);

# Данные взял отсюда; https://base.garant.ru/10180094/
# Можно ещё отсюда взять (но тут начинается с 2013): https://cbr.ru/hd_base/KeyRate/?UniDbQuery.Posted=True&UniDbQuery.From=17.09.2013&UniDbQuery.To=28.12.2024
# Просто скопировал обе таблички и вставил без редактирования в файлик data/bonds.in, остальное должен сделать скрипт, заморачиваться с датами сильно не стал -
# в результате для каждого месяца вывожу ту ставку, которая была на конец месяца.
{
	my $in = 'data/bonds.in';
	my $out = 'data/bonds.txt';

	my $content = do {
		open my $fh, '<', $in
			or die "can't open `$in': $!";
		local $/; <$fh>;
	};

	my ($data, $min_yyyy, $min_mm, $max_yyyy, $max_mm) = ({}, 3000, 12, 0, 0);
	my %ret = $content =~ /(\d{1,2} \S+ \d{4}) г\.\s+(\d+,?\d*)/sg;
	foreach my $k (keys %ret) {
		my $v = $ret{$k};
		$v =~ s/,/./;

		my ($dd, $mname, $yyyy) = split /\s+/, $k;
		my $mm = $month2num{$mname}
			or die "unknown month name `$mname'\n";

		if (not $data->{$yyyy}{$mm}) {
			$data->{$yyyy}{$mm} = {
				last_dd => $dd,
				last_rate => $v,
			};
		} elsif ($data->{$yyyy}{$mm}{last_dd} < $dd) {
			$data->{$yyyy}{$mm}{last_dd} = $dd;
			$data->{$yyyy}{$mm}{last_rate} = $v;
		}

		if ($yyyy < $min_yyyy) {
			$min_yyyy = $yyyy;
			$min_mm = $mm;
		} elsif ($yyyy == $min_yyyy and $mm < $min_mm) {
			$min_mm = $mm;
		}

		if ($yyyy > $max_yyyy) {
			$max_yyyy = $yyyy;
			$max_mm = $mm;
		} elsif ($yyyy == $max_yyyy and $mm > $max_mm) {
			$max_mm = $mm;
		}
	}

	my @now = localtime(time());
	my $current_year = $now[5] + 1900;
	if ($max_yyyy < $current_year) {
		$max_yyyy = $current_year;
	}
	my $current_month = $now[4] + 1;
	if ($max_mm < $current_month) {
		$max_mm = $current_month;
	}

	open my $os, '>', $out
		or die "can't open `$out': $!";
	my $last_rate = 0;
	for (my $yyyy = $min_yyyy; $yyyy <= $max_yyyy; $yyyy++) {
		for (my $mm = $min_mm; $mm <= 12; $mm++) {
			my $rate = $data->{$yyyy}{$mm}{last_rate} // $last_rate;
			printf {$os} "$yyyy/$mm %.2f\n", $rate;

			last if $mm == $max_mm and $yyyy == $max_yyyy;
			$last_rate = $rate;
			$min_mm = 1;
		}
	}
	close $os;

	print "data from `$in' processed and stored into `$out'\n";
}

# Данные беру отсюда:
# - https://www.cbr.ru/statistics/b_sector/credits_deposits_98/
# - ...
# - https://www.cbr.ru/statistics/b_sector/credits_deposits_12/
# - https://cbr.ru/statistics/avgprocstav/?UniDbQuery.Posted=True&UniDbQuery.From=2.07.2009&UniDbQuery.To=2.12.2024
# В качестве результата беру среднее значение за месяц
{
	my $in_old = 'data/deposits.in';
	my $in_new = 'data/deposits.new.in';
	my $out = 'data/deposits.txt';

	my $data = {};
	open my $fh_old, '<', $in_old
		or die "can't open `$in_old': $!";
	while (my $line = <$fh_old>) {
		chomp $line;

		$line =~ s/,/./g;

		my @v = split /\s+/, $line;
		my $yyyy = shift @v;
		die "unexpected numbers of elements [@v]"
			if scalar(@v) != 12;
		for (my $mm = 0; $mm < 12; $mm++) {
			$data->{$yyyy}{$mm + 1} = $v[$mm];
		}
	}

	open my $fh_new, '<', $in_new
		or die "can't open `$in_new': $!";
	my $data_tmp = {};
	while (my $line = <$fh_new>) {
		chomp $line;

		$line =~ s/,/./;

		my ($mm, $yyyy, $rate) = $line =~ /[^.]+\.(\d+)\.(\d+)\s+(\d+\.?\d*)/;
		next if not $rate;

		push @{ $data_tmp->{$yyyy}{$mm + 0} }, $rate;
	}
	foreach my $yyyy (keys %{ $data_tmp }) {
		foreach my $mm (keys %{ $data_tmp->{$yyyy} }) {
			my $items = $data_tmp->{$yyyy}{$mm};

			my $avg = 0;
			foreach my $v (@{ $items }) {
				$avg += $v;
			}
			$avg /= scalar(@{ $items });

			$data->{$yyyy}{$mm} = $avg;
		}
	}

	open my $os, '>', $out;
	foreach my $yyyy (sort { $a <=> $b } keys %{ $data }) {
		foreach my $mm (sort { $a <=> $b } keys %{ $data->{$yyyy} }) {
			my $v = $data->{$yyyy}{$mm};
			printf {$os} "$yyyy/$mm %.2f\n", $v;
		}
	}
	close $os;

	print "data from `$in_old' and `$in_new' processed and stored into `$out'\n";
}

# данные получаем отсюда:
# - 1997 - 2003: https://cbr.ru/hd_base/metall/metall_base_old/?UniDbQuery.Posted=True&UniDbQuery.From=25.03.1997&UniDbQuery.To=04.07.2003&UniDbQuery.P1=1&UniDbQuery.so=1
# - 2003 - 2008: https://cbr.ru/hd_base/metall/metall_base_upto/?UniDbQuery.Posted=True&UniDbQuery.From=05.07.2003&UniDbQuery.To=30.06.2008&UniDbQuery.Gold=true&UniDbQuery.so=1
# - 2008 - ... : https://cbr.ru/hd_base/metall/metall_base_new/?UniDbQuery.Posted=True&UniDbQuery.From=01.07.2008&UniDbQuery.To=29.12.2024&UniDbQuery.Gold=true&UniDbQuery.so=1
#
# усредняем по месяцам и результат сохраняем в data/gldrub.txt
{
	my $out = 'data/gldrub.txt';

	my $data = {};
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_old/?UniDbQuery.Posted=True&UniDbQuery.From=25.03.1997&UniDbQuery.To=04.07.2003&UniDbQuery.P1=1&UniDbQuery.so=1', 1, $data);
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_upto/?UniDbQuery.Posted=True&UniDbQuery.From=05.07.2003&UniDbQuery.To=30.06.2008&UniDbQuery.Gold=true&UniDbQuery.so=1', 0, $data);
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_new/?UniDbQuery.Posted=True&UniDbQuery.From=01.07.2008&UniDbQuery.To=29.12.2024&UniDbQuery.Gold=true&UniDbQuery.so=1', 0, $data);
	store_data($data, $out);

	print "data processed and stored into `$out'\n";
}

# аналогично для серебра, усредняем по месяцам и результат сохраняем в data/slvrub.txt
{
	my $out = 'data/slvrub.txt';

	my $data = {};
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_old/?UniDbQuery.Posted=True&UniDbQuery.From=25.03.1997&UniDbQuery.To=04.07.2003&UniDbQuery.P1=2&UniDbQuery.so=1', 1, $data);
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_upto/?UniDbQuery.Posted=True&UniDbQuery.From=05.07.2003&UniDbQuery.To=30.06.2008&UniDbQuery.Silver=true&UniDbQuery.so=1', 0, $data);
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_new/?UniDbQuery.Posted=True&UniDbQuery.From=01.07.2008&UniDbQuery.To=29.12.2024&UniDbQuery.Silver=true&UniDbQuery.so=1', 0, $data);
	store_data($data, $out);

	print "data processed and stored into `$out'\n";
}

# аналогично для платины, усредняем по месяцам и результат сохраняем в data/pltrub.txt
{
	my $out = 'data/pltrub.txt';

	my $data = {};
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_old/?UniDbQuery.Posted=True&UniDbQuery.From=25.03.1997&UniDbQuery.To=04.07.2003&UniDbQuery.P1=3&UniDbQuery.so=1', 1, $data);
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_upto/?UniDbQuery.Posted=True&UniDbQuery.From=05.07.2003&UniDbQuery.To=30.06.2008&UniDbQuery.Platinum=true&UniDbQuery.so=1', 0, $data);
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_new/?UniDbQuery.Posted=True&UniDbQuery.From=01.07.2008&UniDbQuery.To=29.12.2024&UniDbQuery.Platinum=true&UniDbQuery.so=1', 0, $data);
	store_data($data, $out);

	print "data processed and stored into `$out'\n";
}

# аналогично для палладия, усредняем по месяцам и результат сохраняем в data/pldrub.txt
{
	my $out = 'data/pldrub.txt';

	my $data = {};
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_old/?UniDbQuery.Posted=True&UniDbQuery.From=25.03.1997&UniDbQuery.To=04.07.2003&UniDbQuery.P1=4&UniDbQuery.so=1', 1, $data);
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_upto/?UniDbQuery.Posted=True&UniDbQuery.From=05.07.2003&UniDbQuery.To=30.06.2008&UniDbQuery.Palladium=true&UniDbQuery.so=1', 0, $data);
	cbr_fetch_metal_prices('https://cbr.ru/hd_base/metall/metall_base_new/?UniDbQuery.Posted=True&UniDbQuery.From=01.07.2008&UniDbQuery.To=29.12.2024&UniDbQuery.Palladium=true&UniDbQuery.so=1', 0, $data);
	store_data($data, $out);

	print "data processed and stored into `$out'\n";
}

# Котировки индекса мосбиржи полной доходности, дока как ходить тут: https://www.moex.com/a2920 и тут https://iss.moex.com/iss/reference/439?lang=en
# В результате сохраняем усредненные значение для каждого месяца
{
	my $out = 'data/mcftr.txt';

	my $data = cbr_fetch_security_prices('MCFTR');
	store_data($data, $out);

	print "data processed and stored into `$out'\n";
}

# Взял из апи арсагеры данных по их фонду акций: https://arsagera.ru//files/arsagera_metriki_fonda.pdf
# тоже сохраняем в итоге усредненные данные по месяцам
{
	my $out = 'data/arsagera_fa.txt';

	my $data = arsagera_fetch_prices('fa');
	store_data($data, $out);

	print "data processed and stored into `$out'\n";
}

sub store_data
{
	my ($data, $out) = @_;

	open my $os, '>', $out;
	foreach my $yyyy (sort { $a <=> $b } keys %{ $data }) {
		foreach my $mm (sort { $a <=> $b } keys %{ $data->{$yyyy} }) {
			my $items = $data->{$yyyy}{$mm};

			my $avg = 0;
			foreach my $v (values %{ $items }) {
				$avg += $v;
			}
			$avg /= scalar(keys %{ $items });

			printf {$os} "$yyyy/$mm %.2f\n", $avg;
		}
	}
	close $os;
}

sub arsagera_fetch_prices
{
	my ($fund_code) = @_;

	my @date = localtime(time());
	my $current_yyyy = $date[5] + 1900;

	my $ua = LWP::UserAgent->new(timeout => 30);

	my $data = {};
	for (my $yyyy = 2005; $yyyy <= $current_yyyy; $yyyy++) {
		my $url = "https://arsagera.ru/api/v1/funds/$fund_code/fund-metrics/?from=$yyyy-01-01&to=$yyyy-12-31";
		my $resp = $ua->get($url);
		if (not $resp->is_success()) {
			my $status_line = $resp->status_line();
			my $code = $resp->code();

			die "http request '$url' failed: $status_line (code: $code)";
		}

		my $content = $resp->decoded_content();
		my $json = from_json($content);
		foreach my $row (@{ $json->{data} }) {
			my ($yyyy, $mm, $dd) = split /-/, $row->{date}, 3;
			$data->{$yyyy}{$mm}{$dd} = $row->{nav_per_share};
		}
	}

	return $data;
}

sub cbr_fetch_security_prices
{
	my ($ticker) = @_;

	my $ua = LWP::UserAgent->new(timeout => 30);

	my ($data, $off) = ({}, 0);
	while (1) {
		my $url = "https://iss.moex.com/iss/history/engines/stock/markets/index/securities/$ticker.json?iss.meta=off&history.columns=TRADEDATE,CLOSE&start=$off";
		my $resp = $ua->get($url);
		if (not $resp->is_success()) {
			my $status_line = $resp->status_line();
			my $code = $resp->code();

			die "http request '$url' failed: $status_line (code: $code)";
		}

		my $content = $resp->decoded_content();
		my $json = from_json($content);
		foreach my $row (@{ $json->{history}{data} }) {
			my ($date, $price) = @{ $row };
			my ($yyyy, $mm, $dd) = split /-/, $date, 3;

			$data->{$yyyy}{$mm}{$dd} = $price;
		}

		my ($cursor_off, $cursor_total, $cursor_limit) = @{ $json->{'history.cursor'}{data}[0] };
		$off += $cursor_limit;

		last if $off >= $cursor_total;
	}

	return $data;
}

sub cbr_fetch_metal_prices
{
	my ($url, $is_old_fmt, $data_ref) = @_;

	my $ua = LWP::UserAgent->new(timeout => 30);
	my $resp = $ua->get($url);
	if (not $resp->is_success()) {
		my $status_line = $resp->status_line();
		my $code = $resp->code();

		die "http request '$url' failed: $status_line (code: $code)";
	}

	my $content = $resp->decoded_content();
	my @data = $content =~ m{<td[^>]*>\s*(.+?)\s*</td>}msg;

	my $column_cnt = $is_old_fmt ? 3 : 2;
	while (my @items = splice @data, 0, $column_cnt) {
		my ($dd, $mm, $yyyy) = split /\./, $items[0], 3;
		die "can't parse date from `$items[0]'"
			if not $yyyy;

		my $p_buy = $items[1] // q{0.0};
		$p_buy =~ s/,/./;
		$p_buy =~ s/ //g;

		my $p_sell = $items[2] // q{0.0};
		$p_sell =~ s/,/./;
		$p_sell =~ s/ //g;

		my $p = ($p_sell + 0) || ($p_buy + 0);
		if ($yyyy < 1998) {
			# в 1998 была деноминация, поэтому приводим цены до 98го к общей базе
			$p /= 1000.0;
		}

		$data_ref->{$yyyy}{$mm}{$dd} = $p;
	}
}
