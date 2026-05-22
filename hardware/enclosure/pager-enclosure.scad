// RFMP Pager Enclosure — parametric design for 3D printing
// Open in OpenSCAD: F5 to preview, F6 to render, export STL for printing
//
// Two-part snap/screw enclosure: bottom box + lid
// All dimensions in mm. Adjust parameters below to fit your specific components.

/* [Enclosure] */
wall = 2.5;             // wall thickness
corner_r = 3;           // outer corner radius
tolerance = 0.3;        // printing tolerance (gap between lid and box)

/* [Orange Pi Zero 2W] */
opi_w = 65;             // board width
opi_d = 30;             // board depth
opi_hole_spacing_x = 58;   // mounting hole X spacing
opi_hole_spacing_y = 23;   // mounting hole Y spacing
opi_standoff_h = 5;        // standoff height from floor
opi_board_thick = 1.6;     // PCB thickness
hat_standoff_h = 11;       // stacking standoff (OPi to carrier HAT)

/* [ILI9341 Display] */
disp_pcb_w = 71;        // module PCB width
disp_pcb_d = 52;        // module PCB depth
disp_view_w = 48;       // viewable area width (active LCD)
disp_view_d = 36;       // viewable area depth
disp_thick = 4;         // module total thickness (PCB + LCD)
disp_mount_hole = 2;    // M2 mounting holes
disp_mount_inset = 2.5; // hole center from PCB edge

/* [BBQ20KBD Keyboard] */
kb_w = 75;              // board width
kb_d = 55;              // board depth
kb_thick = 5;           // total thickness including keys
kb_key_area_w = 68;     // exposed key area width
kb_key_area_d = 48;     // exposed key area depth
kb_mount_hole = 2;      // M2 mounting holes
kb_mount_inset = 3;     // hole center from PCB edge

/* [Battery] */
batt_w = 70;            // LiPo width
batt_d = 40;            // LiPo depth
batt_h = 8;             // LiPo thickness
batt_clearance = 1;     // extra room around battery

/* [IP5306 Power Module] */
pwr_w = 25;             // module width
pwr_d = 18;             // module depth
pwr_h = 5;              // module height

/* [USB Ports] */
usbc_w = 9;             // USB-C port width (IP5306 charge input)
usbc_h = 3.5;           // USB-C port height
usba_w = 14;            // USB-A port width (OPi data port → Digirig)
usba_h = 6.5;           // USB-A port height

/* [Assembly Screws] */
screw_d = 3;            // M3 corner screws (lid to box)
screw_boss_d = 7;       // boss outer diameter
screw_boss_inset = 5;   // boss center from inner wall

/* [Computed Dimensions] */
// Internal space must fit: OPi+HAT stack side-by-side with battery
inner_w = max(kb_w, disp_pcb_w, opi_w + pwr_w + 5) + 4;
inner_d = max(kb_d, batt_d + opi_d + 2) + 2;

// Bottom box: OPi stack + battery + clearance
box_inner_h = opi_standoff_h + opi_board_thick + hat_standoff_h +
              opi_board_thick + 3;  // 3mm clearance above HAT

// Lid: display + keyboard mounted on underside
lid_inner_h = max(disp_thick, kb_thick) + 4;

// Outer dimensions
outer_w = inner_w + 2*wall;
outer_d = inner_d + 2*wall;
outer_h_box = box_inner_h + wall;       // bottom box height
outer_h_lid = lid_inner_h + wall;       // lid height
outer_h_total = outer_h_box + outer_h_lid;

/* [Display] */
// Render selector — uncomment one to export
part = "assembly";  // "box", "lid", "assembly"

// Lip height for box/lid alignment
lip_h = 2;
lip_w = 1.2;

// --- Modules ---

module rounded_box(w, d, h, r) {
    hull() {
        for (x = [r, w-r], y = [r, d-r]) {
            translate([x, y, 0])
                cylinder(h=h, r=r, $fn=32);
        }
    }
}

module screw_boss(h, outer_d, hole_d) {
    difference() {
        cylinder(h=h, d=outer_d, $fn=24);
        translate([0, 0, -0.1])
            cylinder(h=h+0.2, d=hole_d, $fn=16);
    }
}

module standoff(h, outer_d, hole_d) {
    difference() {
        cylinder(h=h, d=outer_d, $fn=24);
        translate([0, 0, h - 4])
            cylinder(h=4.1, d=hole_d, $fn=16);
    }
}

module bottom_box() {
    difference() {
        // Outer shell
        rounded_box(outer_w, outer_d, outer_h_box, corner_r);

        // Hollow interior
        translate([wall, wall, wall])
            rounded_box(inner_w, inner_d, outer_h_box, corner_r - wall/2);

        // USB-C cutout (IP5306 charge port) — left side wall
        translate([-0.1, outer_d/2 - usbc_w/2, wall + opi_standoff_h])
            cube([wall+0.2, usbc_w, usbc_h]);

        // OPi USB-A port cutout (Digirig connects here) — right side wall
        opi_usb_z = wall + opi_standoff_h + opi_board_thick;
        translate([outer_w - wall - 0.1, outer_d/2 - usba_w/2, opi_usb_z])
            cube([wall+0.2, usba_w, usba_h]);

        // Ventilation slots — back wall
        for (i = [0:4]) {
            translate([wall + 10 + i*12, outer_d - wall - 0.1, wall + 3])
                cube([8, wall+0.2, 2]);
        }
    }

    // Alignment lip (raised rim on top edge of box)
    translate([wall + tolerance, wall + tolerance, outer_h_box - 0.01])
        difference() {
            rounded_box(inner_w - 2*tolerance, inner_d - 2*tolerance, lip_h, corner_r - wall);
            translate([lip_w, lip_w, -0.1])
                rounded_box(inner_w - 2*tolerance - 2*lip_w, inner_d - 2*tolerance - 2*lip_w, lip_h+0.2, corner_r - wall - lip_w);
        }

    // OPi mounting standoffs (4×, matches 58×23mm hole pattern)
    opi_origin_x = wall + (inner_w - opi_w)/2;
    opi_origin_y = wall + 2;  // OPi toward front of box
    opi_hole_offset_x = (opi_w - opi_hole_spacing_x) / 2;
    opi_hole_offset_y = (opi_d - opi_hole_spacing_y) / 2;

    for (x = [0, opi_hole_spacing_x], y = [0, opi_hole_spacing_y]) {
        translate([opi_origin_x + opi_hole_offset_x + x,
                   opi_origin_y + opi_hole_offset_y + y,
                   wall])
            standoff(opi_standoff_h, 5.5, 2.5);
    }

    // Battery compartment walls — open-ended pocket for slide-out removal
    batt_origin_x = wall + (inner_w - batt_w)/2;
    batt_origin_y = wall + inner_d - batt_d - batt_clearance - 2;
    batt_wall_h = batt_h + 2;

    // Side rails (battery slides along X axis)
    translate([batt_origin_x - 1.5, batt_origin_y, wall])
        cube([1.5, batt_d + 2*batt_clearance, batt_wall_h]);
    translate([batt_origin_x + batt_w + batt_clearance, batt_origin_y, wall])
        cube([1.5, batt_d + 2*batt_clearance, batt_wall_h]);

    // Retaining lip on one end (back) — battery slides out toward front
    translate([batt_origin_x - 1.5, batt_origin_y + batt_d + batt_clearance, wall])
        cube([batt_w + batt_clearance + 3, 1.5, batt_wall_h]);

    // Corner screw bosses (4×, for M3 lid attachment)
    for (x = [wall + screw_boss_inset, outer_w - wall - screw_boss_inset],
         y = [wall + screw_boss_inset, outer_d - wall - screw_boss_inset]) {
        translate([x, y, wall])
            screw_boss(outer_h_box - wall - 0.5, screw_boss_d, screw_d);
    }
}

module lid() {
    difference() {
        // Outer shell
        rounded_box(outer_w, outer_d, outer_h_lid, corner_r);

        // Hollow interior
        translate([wall, wall, wall])
            rounded_box(inner_w, inner_d, outer_h_lid, corner_r - wall/2);

        // Display window cutout (viewable area + 1mm margin)
        disp_cutout_w = disp_view_w + 2;
        disp_cutout_d = disp_view_d + 2;
        disp_cx = outer_w/2;
        disp_cy = wall + 6 + disp_pcb_d/2;  // display toward top of lid
        translate([disp_cx - disp_cutout_w/2, disp_cy - disp_cutout_d/2, -0.1])
            cube([disp_cutout_w, disp_cutout_d, wall + 0.2]);

        // Keyboard window cutout (key area exposed)
        kb_cutout_w = kb_key_area_w + 2;
        kb_cutout_d = kb_key_area_d + 2;
        kb_cx = outer_w/2;
        kb_cy = disp_cy + disp_pcb_d/2 + 4 + kb_d/2;  // keyboard below display
        translate([kb_cx - kb_cutout_w/2, kb_cy - kb_cutout_d/2, -0.1])
            cube([kb_cutout_w, kb_cutout_d, wall + 0.2]);

        // LED light pipe hole (3mm, positioned over D1 on carrier HAT)
        led_x = wall + 10;
        led_y = outer_d - wall - 10;
        translate([led_x, led_y, -0.1])
            cylinder(h=wall+0.2, d=3.2, $fn=16);

        // Corner screw holes (through lid top)
        for (x = [wall + screw_boss_inset, outer_w - wall - screw_boss_inset],
             y = [wall + screw_boss_inset, outer_d - wall - screw_boss_inset]) {
            translate([x, y, -0.1])
                cylinder(h=outer_h_lid+0.2, d=screw_d + 0.3, $fn=16);
            // Countersink on top
            translate([x, y, outer_h_lid - 1.5])
                cylinder(h=1.6, d1=screw_d+0.3, d2=screw_d+3, $fn=16);
        }
    }

    // Display mounting posts (4× M2, on lid underside)
    disp_cx = outer_w/2;
    disp_cy = wall + 6 + disp_pcb_d/2;
    disp_post_h = outer_h_lid - wall - disp_thick - 0.5;

    for (x = [-disp_pcb_w/2 + disp_mount_inset, disp_pcb_w/2 - disp_mount_inset],
         y = [-disp_pcb_d/2 + disp_mount_inset, disp_pcb_d/2 - disp_mount_inset]) {
        translate([disp_cx + x, disp_cy + y, wall])
            standoff(disp_post_h, 4.5, disp_mount_hole);
    }

    // Keyboard mounting posts (4× M2, on lid underside)
    kb_cx = outer_w/2;
    kb_cy = disp_cy + disp_pcb_d/2 + 4 + kb_d/2;
    kb_post_h = outer_h_lid - wall - kb_thick - 0.5;

    for (x = [-kb_w/2 + kb_mount_inset, kb_w/2 - kb_mount_inset],
         y = [-kb_d/2 + kb_mount_inset, kb_d/2 - kb_mount_inset]) {
        translate([kb_cx + x, kb_cy + y, wall])
            standoff(kb_post_h, 4.5, kb_mount_hole);
    }
}

module assembly() {
    // Bottom box
    color("DimGray") bottom_box();

    // Lid (flipped and placed on top)
    translate([0, 0, outer_h_box + lip_h + 1])
        color("SlateGray", 0.7)
            mirror([0, 0, 1])
                translate([0, 0, -outer_h_lid])
                    lid();

    // Ghost components for visualization
    opi_origin_x = wall + (inner_w - opi_w)/2;
    opi_origin_y = wall + 2;

    // OPi board
    translate([opi_origin_x, opi_origin_y, wall + opi_standoff_h])
        color("Green", 0.5) cube([opi_w, opi_d, opi_board_thick]);

    // Carrier HAT
    translate([opi_origin_x, opi_origin_y,
               wall + opi_standoff_h + opi_board_thick + hat_standoff_h])
        color("Blue", 0.5) cube([opi_w, opi_d, opi_board_thick]);

    // Battery
    batt_origin_x = wall + (inner_w - batt_w)/2;
    batt_origin_y = wall + inner_d - batt_d - batt_clearance - 2;
    translate([batt_origin_x, batt_origin_y, wall])
        color("Orange", 0.5) cube([batt_w, batt_d, batt_h]);
}

// --- Render ---

if (part == "box") {
    bottom_box();
} else if (part == "lid") {
    lid();
} else {
    assembly();
}
