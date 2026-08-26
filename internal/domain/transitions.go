package domain

import "time"

func (c *QuarantineCase) ensureOpen(action string) error {
	if c.Status.Closed() {
		return StateError(c.Status, action)
	}
	return nil
}

func (c *QuarantineCase) SubmitRisk(level RiskLevel) error {
	if err := c.ensureOpen("提交风险审查"); err != nil {
		return err
	}
	if c.Status != StatusDraft && c.Status != StatusReturned {
		return StateError(c.Status, "提交风险审查")
	}
	c.Status, c.RiskLevel = StatusPendingReview, level
	return nil
}

func (c *QuarantineCase) Review(approved bool) error {
	if err := c.ensureOpen("审核风险方案"); err != nil {
		return err
	}
	if c.Status != StatusPendingReview {
		return StateError(c.Status, "审核风险方案")
	}
	if approved {
		c.Status = StatusApproved
	} else {
		c.Status = StatusReturned
	}
	return nil
}

func (c *QuarantineCase) StartObservation(days int, now time.Time) error {
	if err := c.ensureOpen("启动隔离观察"); err != nil {
		return err
	}
	if c.Status != StatusApproved {
		return StateError(c.Status, "启动隔离观察")
	}
	if days < 1 {
		return FieldError("quarantine_days", "隔离期限必须大于零")
	}
	start := dateOnly(now)
	expected := start.AddDate(0, 0, days)
	c.ObservationStartedAt, c.ExpectedReleaseAt, c.Status = &start, &expected, StatusObserving
	return nil
}

func (c *QuarantineCase) AddObservation() error {
	if err := c.ensureOpen("登记观察"); err != nil {
		return err
	}
	if c.Status != StatusObserving && c.Status != StatusRestricted {
		return StateError(c.Status, "登记观察")
	}
	return nil
}

func (c *QuarantineCase) Restrict() error {
	if err := c.ensureOpen("登记偏差"); err != nil {
		return err
	}
	if c.Status != StatusObserving && c.Status != StatusRestricted {
		return StateError(c.Status, "登记偏差")
	}
	c.Status = StatusRestricted
	return nil
}

func (c *QuarantineCase) Resume(hasOtherOpen bool) error {
	if err := c.ensureOpen("恢复观察"); err != nil {
		return err
	}
	if c.Status != StatusRestricted {
		return StateError(c.Status, "恢复观察")
	}
	if hasOtherOpen {
		return NewError(CodeState, "仍有未关闭偏差，不能恢复观察")
	}
	c.Status = StatusObserving
	return nil
}

func (c *QuarantineCase) MarkEligible(result EligibilityResult) error {
	if err := c.ensureOpen("申请结论"); err != nil {
		return err
	}
	if c.Status == StatusRestricted {
		return NewError(CodeState, "受限期间不能申请结论")
	}
	if c.Status != StatusObserving && c.Status != StatusEligible {
		return StateError(c.Status, "申请结论")
	}
	if !result.Eligible {
		return NewError(CodeState, "放行资格检查未通过")
	}
	c.Status = StatusEligible
	return nil
}

func (c *QuarantineCase) Close(outcome Outcome, now time.Time) error {
	if err := c.ensureOpen("作出最终决定"); err != nil {
		return err
	}
	if outcome == OutcomeRelease && c.Status != StatusEligible {
		return StateError(c.Status, "作出放行决定")
	}
	if outcome == OutcomeTerminate && c.Status != StatusObserving && c.Status != StatusRestricted && c.Status != StatusEligible {
		return StateError(c.Status, "作出终止决定")
	}
	if outcome == OutcomeRelease {
		c.Status = StatusReleased
	} else {
		c.Status = StatusTerminated
	}
	t := now.UTC()
	c.ClosedAt = &t
	return nil
}
