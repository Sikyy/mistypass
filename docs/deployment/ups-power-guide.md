# UPS Power Guide for MistyPass Deployments

> Power backup recommendations for physical access control systems.
>
> Last updated: 2026-05-01

---

## Why UPS Is Critical for Access Control

A power outage in a building with electronic access control can have immediate, serious consequences:

- **People locked out** -- employees, residents, or emergency responders cannot enter the building.
- **People locked in** -- fire safety codes often require egress at all times; power loss must not trap occupants.
- **Security gaps** -- fail-safe locks release during power loss, leaving doors unsecured until power returns.
- **Data loss** -- gateways may lose queued access events or corrupt local configuration cache.
- **Hardware damage** -- sudden power loss can damage flash storage on gateway devices over time.

A properly sized UPS ensures the access control system continues operating during outages and gives administrators time to respond.

---

## What to Put on UPS

### Must-have (critical path)

Every component in the access decision chain must have battery backup:

| Component | Why |
|-----------|-----|
| MistyPass Gateway | Makes access decisions, actuates locks |
| PoE Switch | Powers readers and some locks via Ethernet |
| Network switch / router | Gateway needs network to verify credentials online |
| Electric locks | Must remain powered to maintain lock state |
| Door controllers | If separate from gateway |

### Nice-to-have

| Component | Why |
|-----------|-----|
| Internet modem / ONT | Maintains cloud connectivity during outage |
| Surveillance cameras | Maintains visual monitoring |
| Intercom | Allows visitors to request access manually |

### Do not put on UPS

- HVAC systems, general lighting, or office equipment -- these draw too much power and will drain the UPS quickly.

---

## Lock Behavior During Power Loss

Understanding lock types is essential for correct UPS planning.

### Fail-Safe (locked when powered, unlocked when power lost)

- The lock **releases** when power is cut.
- Common on interior doors and fire exits where building codes require free egress.
- Without UPS, a power outage unlocks the door -- a security risk.
- UPS keeps the lock energized and secure.

### Fail-Secure (unlocked when powered, locked when power lost)

- The lock **remains locked** when power is cut.
- Common on exterior doors, server rooms, and high-security areas.
- Without UPS, people may be locked in or locked out.
- UPS maintains the gateway's ability to grant access via credential presentation.

### Recommendation

| Door type | Lock mode | UPS priority |
|-----------|-----------|--------------|
| Main entrance | Fail-secure | High -- people locked out |
| Fire exit / stairwell | Fail-safe | High -- door unsecured |
| Interior office | Fail-safe | Medium |
| Server room | Fail-secure | High -- access blocked |

All lock types benefit from UPS. Fail-safe locks need UPS to stay secure. Fail-secure locks need UPS so the gateway can still grant access.

---

## UPS Sizing

### Step 1: Calculate Total Power Draw

Measure or look up the wattage of each component on the UPS.

| Component | Typical power draw |
|-----------|--------------------|
| MistyPass Gateway | 5-15 W |
| PoE switch (8-port) | 60-150 W |
| Electric strike (fail-safe) | 3-8 W per lock (continuous) |
| Mag lock | 6-12 W per lock (continuous) |
| Network switch | 10-30 W |
| Internet modem/ONT | 10-20 W |

**Example: small office (2 doors)**

| Component | Watts |
|-----------|-------|
| Gateway | 10 W |
| PoE switch (8-port) | 80 W |
| 2x mag locks | 20 W |
| Network switch | 15 W |
| **Total** | **125 W** |

### Step 2: Choose Runtime

| Scenario | Minimum runtime |
|----------|-----------------|
| Generator backup available | 15 minutes (bridge until generator starts) |
| No generator, urban area | 30-60 minutes |
| No generator, remote site | 2-4 hours |

### Step 3: Size the UPS

```
Required UPS capacity (VA) = Total Watts / Power Factor / Derating

Power Factor: typically 0.6-0.8 for UPS (use 0.7 as safe default)
Derating: 0.8 (run UPS at max 80% capacity for longevity)
```

**Example:**

```
125 W / 0.7 / 0.8 = 223 VA  (minimum UPS rating for instant capacity)
```

For runtime, check the UPS manufacturer's runtime chart at the calculated wattage. A 125 W load typically gets:
- **600 VA UPS**: ~15 minutes
- **1000 VA UPS**: ~30 minutes
- **1500 VA UPS**: ~60 minutes

### Step 4: Select UPS Type

| Type | Use case |
|------|----------|
| Standby (offline) | Small sites, 1-2 doors, budget installs |
| Line-interactive | Recommended for most access control installs |
| Online (double-conversion) | Mission-critical facilities, clean power required |

---

## Gateway Battery Backup

For gateways installed in remote locations (e.g., detached buildings, gate posts), a local 12V UPS provides dedicated backup independent of the building's main UPS.

### Recommended Specification

| Parameter | Minimum | Recommended |
|-----------|---------|-------------|
| Voltage | 12 V DC | 12 V DC |
| Capacity | 7 Ah (84 Wh) | 12-18 Ah (144-216 Wh) |
| Runtime at 10 W | ~8 hours | ~14-21 hours |
| Charger | Built-in float charger | Built-in with overcharge protection |

A 12V 7Ah sealed lead-acid (SLA) battery with a float charger provides roughly 8 hours of runtime for a gateway drawing 10 W. Lithium-iron-phosphate (LiFePO4) batteries offer longer cycle life in the same form factor.

### Wiring

```
AC Mains --> 12V Power Supply --> Battery Charger
                                      |
                                  12V Battery
                                      |
                                  Gateway (12V DC in)
```

The charger maintains the battery at float voltage. If mains power fails, the battery seamlessly provides power to the gateway.

---

## Monthly UPS Test Procedure

Perform this check once per month to verify the UPS will work when needed.

### Checklist

| Step | Action | Expected result |
|------|--------|-----------------|
| 1 | Visual inspection | No warning LEDs, no swelling or leaking batteries, no unusual heat |
| 2 | Check UPS management interface or LCD | Battery health OK, load within capacity, no fault codes |
| 3 | Simulate power failure (unplug UPS from mains) | UPS alarm sounds, connected equipment stays powered |
| 4 | Run on battery for 5 minutes | Verify all components operate normally on battery |
| 5 | Verify MistyPass gateway stays online | Check gateway heartbeat in admin dashboard during battery test |
| 6 | Reconnect mains power | UPS returns to mains, begins recharging |
| 7 | Record results | Log date, runtime observed, battery health status |

### Battery Replacement Schedule

| Battery type | Expected life | Replacement trigger |
|--------------|---------------|---------------------|
| SLA (lead-acid) | 3-5 years | Runtime drops below 50% of rated, or battery swelling |
| LiFePO4 | 7-10 years | Runtime drops below 70% of rated |

### UPS Monitoring Integration

If the UPS supports SNMP or USB monitoring (most rack-mount units do), connect it to your monitoring system. Alert on:

- Battery low (< 25% charge)
- UPS on battery (power outage detected)
- Battery replacement needed
- UPS overload

---

## Summary

| Guideline | Recommendation |
|-----------|----------------|
| Minimum runtime | 30 minutes without generator |
| UPS type | Line-interactive for most sites |
| Gateway backup | 12V SLA or LiFePO4, 7Ah minimum |
| Lock coverage | Both fail-safe and fail-secure need UPS |
| Testing | Monthly simulated power failure |
| Battery replacement | Every 3-5 years (SLA), check annually |
