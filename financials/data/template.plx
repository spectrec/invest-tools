{
	financials_url => "https://...",

	years => [ 2017, 2018, 2019, 2020, 2021, 2022 ],

	# https://www.moex.com/s26
	stock_count => [ 1, 1, 1, 1, 1, 1 ],

	balance => {
		assets => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],
		current_assets => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],

		liabilities => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],
		current_liabilities => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],

		debt => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],
	},

	income => {
		revenue => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],
		operating_income => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],
		interest_expences => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],

		net_income => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],
		adj_net_income => [ undef, undef, undef, undef, undef, undef ],
	},

	cache_flow => {
		net_operation_cf => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],
		net_investing_cf => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],

		dividends => [ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 ],
	},
}
