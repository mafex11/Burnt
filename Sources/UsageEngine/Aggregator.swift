import Foundation

public enum Aggregator {
    private static var cal: Calendar { Calendar(identifier: .gregorian) }

    private static func parse(_ period: String) -> Date? {
        let parts = period.split(separator: "-").compactMap { Int($0) }
        guard parts.count == 3 else { return nil }
        var c = DateComponents(); c.year = parts[0]; c.month = parts[1]; c.day = parts[2]
        return cal.date(from: c)
    }

    private static func key(_ date: Date) -> String {
        let c = cal.dateComponents([.year, .month, .day], from: date)
        return String(format: "%04d-%02d-%02d", c.year!, c.month!, c.day!)
    }

    public static func summary(from report: CcusageReport, referenceDate: Date,
                               byProject: [ProjectSlice] = []) -> Summary {
        let today = cal.startOfDay(for: referenceDate)
        let todayKey = key(today)
        // Rolling 7 days inclusive: today and the 6 prior days.
        let weekStart = cal.date(byAdding: .day, value: -6, to: today)!

        func inWeek(_ d: DailyUsage) -> Bool {
            guard let date = parse(d.period) else { return false }
            return date >= weekStart && date <= today
        }

        var todayTotals = Totals()
        if let d = report.daily.first(where: { $0.period == todayKey }) {
            todayTotals = totals(for: d)
        }

        let weekDays = report.daily.filter(inWeek)

        var weekTotals = Totals()
        for d in weekDays { weekTotals = add(weekTotals, totals(for: d)) }

        // Sparkline series: last 14 days, oldest→newest, zero-filled. Sourced from
        // ALL daily rows (not just the 7-day week window) so it spans two weeks.
        let costByDay = Dictionary(grouping: report.daily, by: { $0.period })
            .mapValues { $0.reduce(0) { $0 + $1.totalCost } }
        var points: [DayPoint] = []
        for offset in stride(from: 13, through: 0, by: -1) {
            let date = cal.date(byAdding: .day, value: -offset, to: today)!
            let k = key(date)
            points.append(DayPoint(date: k, cost: costByDay[k] ?? 0))
        }

        // 84-day series for the heatmap, same zero-filled construction over all rows.
        let costByDayAll = Dictionary(grouping: report.daily, by: { $0.period })
            .mapValues { $0.reduce(0) { $0 + $1.totalCost } }
        var heatPoints: [DayPoint] = []
        for offset in stride(from: 83, through: 0, by: -1) {
            let date = cal.date(byAdding: .day, value: -offset, to: today)!
            let k = key(date)
            heatPoints.append(DayPoint(date: k, cost: costByDayAll[k] ?? 0))
        }

        var toolCost: [Tool: (Double, Int)] = [:]
        var modelAgg: [String: (Tool, Double, Int)] = [:]
        var cacheSavings = 0.0
        for d in weekDays {
            for m in d.modelBreakdowns {
                let tool = ToolClassifier.tool(forModel: m.modelName)
                let total = m.inputTokens + m.outputTokens + m.cacheCreationTokens + m.cacheReadTokens
                let t = toolCost[tool] ?? (0, 0)
                toolCost[tool] = (t.0 + m.cost, t.1 + total)
                let ma = modelAgg[m.modelName] ?? (tool, 0, 0)
                modelAgg[m.modelName] = (tool, ma.1 + m.cost, ma.2 + total)
                cacheSavings += CachePricing.estimatedSavings(cacheReadTokens: m.cacheReadTokens, model: m.modelName)
            }
        }

        let byTool = toolCost.map { ToolSlice(tool: $0.key, cost: $0.value.0, totalTokens: $0.value.1) }
            .sorted { $0.cost > $1.cost }
        let byModel = modelAgg.map { ModelSlice(modelName: $0.key, tool: $0.value.0, cost: $0.value.1, totalTokens: $0.value.2) }
            .sorted { $0.cost > $1.cost }

        let month = monthToDate(report, today)
        let all = allTime(report)
        let avg = weekTotals.cost / 7.0
        let last = lastWeekTotals(report, today)
        let trend: Double? = last.cost == 0 ? nil : (weekTotals.cost - last.cost) / last.cost
        let projected = projectedToday(todayTotals.cost, referenceDate)

        return Summary(today: todayTotals, thisWeek: weekTotals, weekByDay: points,
            heatmapDays: heatPoints, byTool: byTool, byModel: byModel, cacheSavings: cacheSavings,
            monthToDate: month, allTime: all, avgPerDay: avg, lastWeek: last,
            weekTrend: trend, projectedToday: projected, byProject: byProject, generatedAt: referenceDate)
    }

    private static func totals(for d: DailyUsage) -> Totals {
        Totals(cost: d.totalCost, inputTokens: d.inputTokens, outputTokens: d.outputTokens,
            cacheCreationTokens: d.cacheCreationTokens, cacheReadTokens: d.cacheReadTokens,
            totalTokens: d.totalTokens)
    }

    private static func add(_ a: Totals, _ b: Totals) -> Totals {
        Totals(cost: a.cost + b.cost, inputTokens: a.inputTokens + b.inputTokens,
            outputTokens: a.outputTokens + b.outputTokens,
            cacheCreationTokens: a.cacheCreationTokens + b.cacheCreationTokens,
            cacheReadTokens: a.cacheReadTokens + b.cacheReadTokens,
            totalTokens: a.totalTokens + b.totalTokens)
    }

    private static func monthToDate(_ report: CcusageReport, _ today: Date) -> Totals {
        let comps = cal.dateComponents([.year, .month], from: today)
        var t = Totals()
        for d in report.daily {
            guard let date = parse(d.period) else { continue }
            let dc = cal.dateComponents([.year, .month], from: date)
            if dc.year == comps.year && dc.month == comps.month {
                t = add(t, totals(for: d))
            }
        }
        return t
    }

    private static func allTime(_ report: CcusageReport) -> Totals {
        let x = report.totals
        return Totals(cost: x.totalCost, inputTokens: x.inputTokens, outputTokens: x.outputTokens,
            cacheCreationTokens: x.cacheCreationTokens, cacheReadTokens: x.cacheReadTokens,
            totalTokens: x.totalTokens)
    }

    private static func lastWeekTotals(_ report: CcusageReport, _ today: Date) -> Totals {
        let end = cal.date(byAdding: .day, value: -7, to: today)!
        let start = cal.date(byAdding: .day, value: -13, to: today)!
        var t = Totals()
        for d in report.daily {
            guard let date = parse(d.period) else { continue }
            if date >= start && date <= end { t = add(t, totals(for: d)) }
        }
        return t
    }

    private static func projectedToday(_ todayCost: Double, _ referenceDate: Date) -> Double? {
        let startOfDay = cal.startOfDay(for: referenceDate)
        let elapsed = referenceDate.timeIntervalSince(startOfDay)
        let fraction = elapsed / 86_400.0
        guard fraction >= 0.1 else { return nil }
        return todayCost / fraction
    }
}
