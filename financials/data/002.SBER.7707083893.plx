{
	financials_url => "https://e-disclosure.ru/portal/files.aspx?id=3043&type=4",

	years => [ 2017, 2018, 2019, 2020, 2021, 2022 ],

	# https://www.moex.com/s26
	stock_count => [ 22_586_948_000, 22_586_948_000, 22_586_948_000, 22_586_948_000, 22_586_948_000, 22_586_948_000 ],

	balance => {
		assets => [ 27_112.2*1e9, 31_197.5*1e9, 29_959.7*1e9, 36_016*1e9, 41_165.5*1e9, 41_871.8*1e9 ],
		liabilities => [ 23_676.2*1e9, 27_341.7*1e9, 25_473.0*1e9, 30_969.5*1e9, 35_521*1e9, 36_057*1e9 ],
	},

	income => {
		revenue => [ 2_746*1e9, 2_827.2*1e9, 3_183*1e9, 3_228.4*1e9, (1_759.4+898.6)*1e9, (1_874.8+940.6)*1e9 ],
		operating_income => [ 0.0, 0.0, 0.0, 0.0, 2_289.6*1e9, 1_386.7*1e9 ],
		interest_expences => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ], # не считаю для банков

		net_income => [ 750.4*1e9, 832.9*1e9, 844.9*1e9, 761.1*1e9, 1_245.9*1e9, 270.5*1e9 ],
		adj_net_income => [ undef, undef, undef, undef, undef, undef ],
	},

	cache_flow => {
		net_operation_cf => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ], # нет в отчете за 2022
		net_investing_cf => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ], # нет в отчете за 2022

		dividends => [ 134.7*1e9, 268.5*1e9, 356.6*1e9, 421.2*1e9, 419.4*1e9, 0.0 ], # нет в отчете за 2022
	},
}
