#!/usr/bin/env python3
"""Independent analytic + complex nodal check of the pinned ngspice RC netlist.
Does NOT run ngspice, Spectre, or any commercial EDA. Outputs labeled references.
"""
import cmath, csv, hashlib, json, math
from pathlib import Path
root = Path(__file__).resolve().parents[1]
source = root / 'tests/fixtures/ngspice/rc-meas-ac.sp'
text = source.read_text()
components = {'R1': 1000.0, 'R2': 1000.0, 'C1': 1e-9, 'C2': 500e-12}
for line in ['R1 in mid1 1k', 'c1 mid1 gnd 1n', 'C2 mid1 out 500p', 'R2 out gnd 1k']:
    assert line in text, 'Upstream topology changed'
r1,r2,c1,c2 = (components[x] for x in ['R1','R2','C1','C2'])
rows=[]; worst=0
for i in range(41):
    f=1e3*10**(i/10.0); s=2j*math.pi*f
    a=1/r1+s*(c1+c2); b=-s*c2; d=1/r2+s*c2
    nodal=(-b/r1)/(a*d-b*b)
    closed=(s*r2*c2)/(1+s*(r1*c1+r1*c2+r2*c2)+s*s*r1*r2*c1*c2)
    error=abs(nodal-closed); worst=max(worst,error); assert error<1e-12
    for metric,value,unit in [('gain',20*math.log10(abs(nodal)),'dB'),('phase',cmath.phase(nodal),'rad')]:
        rows.append([format(f,'.17g')+'Hz','nominal',metric,format(value,'.17g'),unit,'ok','',''])
output=root/'tests/fixtures/ngspice/analytic-reference.csv'
with output.open('w',newline='') as out:
    writer=csv.writer(out);writer.writerow(['point','corner','metric','value','unit','status','min','max']);writer.writerows(rows)
report={'source':'https://github.com/ngspice/ngspice/blob/032b1c32c4dbad45ff132bcfac1dbecadbd8abb0/examples/measure/rc-meas-ac.sp','commit':'032b1c32c4dbad45ff132bcfac1dbecadbd8abb0','inputSHA256':hashlib.sha256(source.read_bytes()).hexdigest(),'outputSHA256':hashlib.sha256(output.read_bytes()).hexdigest(),'method':'independent two-node AC equations vs closed-form transfer function','frequencies':41,'metrics':len(rows),'maxAbsoluteComplexError':worst,'notVerified':['ngspice execution','Virtuoso execution','Spectre execution']}
(root/'docs/validation/reference-rc.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report,indent=2))
