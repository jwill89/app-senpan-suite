using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Numerics;
using System.Threading.Tasks;
using Dalamud.Bindings.ImGui;
using Dalamud.Interface;
using SenpanCompanion.Api;

namespace SenpanCompanion.Windows;

/// <summary>
/// Gpose Booth operator panel (Festival -> Gpose Booth): the photographer works a
/// booth's sign-ups from in game. Sign-ups themselves are made on the website only;
/// here staff mark them completed, a no-show or canceled, record them paid, undo a
/// mistake, call the next person up (at a basic booth), and put someone from a
/// timeslot booth's waitlist into a slot a no-show freed.
///
/// A call alerts the person on their festival dashboard and booth page until they
/// dismiss it. Where each sign-up stands - its place in line, the waitlist moving up
/// into a freed place, the times a waitlisted shoot fits - is the server's to decide:
/// every change is a PATCH (or the call's POST), and the list is read again after it,
/// since one change can move others. A gpose-booths change from anywhere marks the
/// page stale.
/// </summary>
internal sealed class GposeTab : TabBase
{
    private readonly SenpanApi api;

    private List<GposeBooth> booths = new();
    private List<GposeSignup> signups = new();
    private long selectedBoothId;

    public GposeTab(SenpanApi api) => this.api = api;

    private GposeBooth? Selected => this.booths.FirstOrDefault(b => b.Id == this.selectedBoothId);

    /// <summary>Reads the booths, and the open booth's sign-ups.</summary>
    protected override async Task LoadAsync()
    {
        var list = await this.api.ListGposeBoothsAsync();
        var id = this.selectedBoothId;
        if (id == 0 || list.Booths.All(b => b.Id != id))
            id = list.Booths.FirstOrDefault(b => b.Status == GposeBoothStatus.Open)?.Id ?? 0;
        var rows = id == 0 ? new List<GposeSignup>() : (await this.api.ListGposeSignupsAsync(id)).Signups.ToList();
        await Apply(() =>
        {
            this.booths = list.Booths.ToList();
            this.selectedBoothId = id;
            this.signups = rows;
        });
    }

    public void Draw()
    {
        this.DrawStatusLine();

        this.DrawBoothPicker();
        ImGui.SameLine();
        if (Ui.Button("Refresh##gpose"))
            this.Run(this.LoadAsync);

        var booth = this.Selected;
        if (booth == null)
        {
            ImGui.TextDisabled(this.Busy ? "Loading..." : "No gpose booths yet. Make one on the website.");
            return;
        }

        var timeslot = booth.Mode == GposeBoothMode.Timeslot;
        UiText.WrappedDisabled(timeslot
            ? "Bookings in time order. Mark each one as it happens; a no-show frees its slot for the waitlist."
            : "Sign-ups in the order they came. Call the next person up - it alerts them on their " +
              "festival dashboard - and mark each one done.");
        ImGui.TextUnformatted(CountsText(booth));

        var booked = this.signups.Where(s => s.Status == GposeSignupStatus.Waiting && !s.Waitlisted).ToList();
        if (timeslot)
            booked = booked.OrderBy(s => s.SlotStart).ToList();
        var waitlist = this.signups.Where(s => s.Status == GposeSignupStatus.Waiting && s.Waitlisted).ToList();
        var finished = this.signups.Where(s => s.Status != GposeSignupStatus.Waiting).ToList();

        Ui.Section(FontAwesomeIcon.CameraRetro, $"{(timeslot ? "Booked" : "In Line")} ({booked.Count})");
        if (booked.Count == 0)
            ImGui.TextDisabled(timeslot ? "No bookings waiting." : "Nobody in line.");
        else
            this.DrawBooked(booked, timeslot);

        if (booth.MaxWaitlist > 0 || waitlist.Count > 0)
        {
            Ui.Section(FontAwesomeIcon.ClipboardList, $"Waitlist ({waitlist.Count})");
            if (waitlist.Count == 0)
                ImGui.TextDisabled("Nobody on the waitlist.");
            else
                this.DrawWaitlist(waitlist, timeslot);
        }

        if (finished.Count > 0 && Ui.CollapsingSection(FontAwesomeIcon.CheckCircle, $"Finished ({finished.Count})", "gposefinished", false))
            this.DrawFinished(finished, timeslot);
    }

    private void DrawBoothPicker()
    {
        var current = this.Selected;
        ImGui.SetNextItemWidth(280);
        if (!ImGui.BeginCombo("##gposepick", current != null ? BoothLabel(current) : "Select booth..."))
            return;
        foreach (var booth in this.booths)
        {
            if (ImGui.Selectable($"{BoothLabel(booth)}##gb{booth.Id}", booth.Id == this.selectedBoothId))
                this.SelectBooth(booth.Id);
        }
        ImGui.EndCombo();
    }

    private void SelectBooth(long id)
    {
        this.selectedBoothId = id;
        this.signups = new List<GposeSignup>();
        this.Run(() => this.ReloadSignupsAsync(id));
    }

    private async Task ReloadSignupsAsync(long id)
    {
        var rows = (await this.api.ListGposeSignupsAsync(id)).Signups.ToList();
        // Only a read that still matches the selection is shown, so no button acts on
        // a booth that isn't on screen.
        await Apply(() =>
        {
            if (this.selectedBoothId == id)
                this.signups = rows;
        });
    }

    private void DrawBooked(List<GposeSignup> rows, bool timeslot)
    {
        if (!ImGui.BeginTable("gposebooked", 5, ImGuiTableFlags.Borders | ImGuiTableFlags.RowBg | ImGuiTableFlags.Resizable))
            return;
        ImGui.TableSetupColumn(timeslot ? "Time" : "#", ImGuiTableColumnFlags.WidthFixed, timeslot ? 150 : 30);
        ImGui.TableSetupColumn("Character");
        ImGui.TableSetupColumn("Type");
        ImGui.TableSetupColumn("Paid", ImGuiTableColumnFlags.WidthFixed, 45);
        ImGui.TableSetupColumn("Actions", ImGuiTableColumnFlags.WidthFixed, timeslot ? 190 : 230);
        ImGui.TableHeadersRow();
        foreach (var s in rows)
        {
            ImGui.TableNextRow();
            ImGui.TableNextColumn();
            ImGui.TextUnformatted(timeslot ? SlotText(s) : s.Position.ToString(CultureInfo.InvariantCulture));
            this.DrawCommonCells(s);
            ImGui.TableNextColumn();
            if (!timeslot)
            {
                if (ImGui.SmallButton($"Call##call{s.Id}"))
                    this.Call(s);
                Ui.ItemTooltip("Alert them on their festival dashboard and booth page");
                ImGui.SameLine();
            }
            if (ImGui.SmallButton($"Done##done{s.Id}"))
                this.SetStatus(s, GposeSignupChangeStatus.Completed);
            ImGui.SameLine();
            if (ImGui.SmallButton($"No-Show##ns{s.Id}"))
                this.SetStatus(s, GposeSignupChangeStatus.No_show);
            ImGui.SameLine();
            if (ImGui.SmallButton($"Cancel##cx{s.Id}"))
                this.SetStatus(s, GposeSignupChangeStatus.Canceled);
        }
        ImGui.EndTable();
    }

    private void DrawWaitlist(List<GposeSignup> rows, bool timeslot)
    {
        if (!ImGui.BeginTable("gposewaitlist", 5, ImGuiTableFlags.Borders | ImGuiTableFlags.RowBg | ImGuiTableFlags.Resizable))
            return;
        ImGui.TableSetupColumn("#", ImGuiTableColumnFlags.WidthFixed, 30);
        ImGui.TableSetupColumn("Character");
        ImGui.TableSetupColumn("Type");
        ImGui.TableSetupColumn("Paid", ImGuiTableColumnFlags.WidthFixed, 45);
        ImGui.TableSetupColumn("Actions", ImGuiTableColumnFlags.WidthFixed, 160);
        ImGui.TableHeadersRow();
        foreach (var s in rows)
        {
            ImGui.TableNextRow();
            ImGui.TableNextColumn();
            ImGui.TextUnformatted(s.Position.ToString(CultureInfo.InvariantCulture));
            this.DrawCommonCells(s);
            ImGui.TableNextColumn();
            if (timeslot)
                this.DrawAssign(s);
            else if (ImGui.SmallButton($"Call##call{s.Id}"))
                this.Call(s);
            ImGui.SameLine();
            if (ImGui.SmallButton($"Cancel##cx{s.Id}"))
                this.SetStatus(s, GposeSignupChangeStatus.Canceled);
        }
        ImGui.EndTable();
    }

    /// <summary>
    /// Puts a waitlisted sign-up into a slot: a popup listing the times its shoot fits
    /// now, as the server worked them out - a no-show's freed slot under way among them.
    /// </summary>
    private void DrawAssign(GposeSignup s)
    {
        var popup = $"##assign{s.Id}";
        if (s.AssignableStarts.Count == 0)
        {
            ImGui.TextDisabled("No slot fits");
            return;
        }
        if (ImGui.SmallButton($"Assign##as{s.Id}"))
            ImGui.OpenPopup(popup);
        Ui.ItemTooltip("Give them a slot their shoot fits");
        if (!ImGui.BeginPopup(popup))
            return;
        ImGui.TextUnformatted($"A slot for {s.CharacterName}");
        ImGui.Separator();
        foreach (var at in s.AssignableStarts)
        {
            if (ImGui.Selectable($"{LocalTime(at)}##at{s.Id}{at}") &&
                DateTimeOffset.TryParse(at, CultureInfo.InvariantCulture, DateTimeStyles.AssumeUniversal, out var start))
            {
                this.Change(s, new GposeSignupChange { SlotStart = start });
            }
        }
        ImGui.EndPopup();
    }

    private void DrawFinished(List<GposeSignup> rows, bool timeslot)
    {
        if (!ImGui.BeginTable("gposefinished", 5, ImGuiTableFlags.Borders | ImGuiTableFlags.RowBg | ImGuiTableFlags.Resizable))
            return;
        ImGui.TableSetupColumn("Status", ImGuiTableColumnFlags.WidthFixed, 85);
        ImGui.TableSetupColumn("Character");
        ImGui.TableSetupColumn("Type");
        ImGui.TableSetupColumn("Paid", ImGuiTableColumnFlags.WidthFixed, 45);
        ImGui.TableSetupColumn(timeslot ? "Time" : "Actions", ImGuiTableColumnFlags.WidthFixed, timeslot ? 200 : 60);
        ImGui.TableHeadersRow();
        foreach (var s in rows)
        {
            ImGui.TableNextRow();
            ImGui.TableNextColumn();
            ImGui.TextColored(StatusColor(s.Status), StatusWord(s.Status));
            this.DrawCommonCells(s);
            ImGui.TableNextColumn();
            if (ImGui.SmallButton($"Undo##undo{s.Id}"))
                this.SetStatus(s, GposeSignupChangeStatus.Waiting);
            Ui.ItemTooltip("Put it back as waiting");
            if (timeslot && s.SlotStart != null)
            {
                ImGui.SameLine();
                ImGui.TextDisabled(SlotText(s));
            }
        }
        ImGui.EndTable();
    }

    /// <summary>The character, type and paid cells every table shares.</summary>
    private void DrawCommonCells(GposeSignup s)
    {
        ImGui.TableNextColumn();
        ImGui.TextUnformatted($"{s.CharacterName} @ {s.World}");
        if (s.CalledAt != null && s.Status == GposeSignupStatus.Waiting)
        {
            ImGui.SameLine();
            Ui.Badge("Called", Ui.WarnColor);
        }
        ImGui.TableNextColumn();
        ImGui.TextUnformatted(s.Price > 0 ? $"{s.TypeName} ({Gil(s.Price)})" : s.TypeName);
        ImGui.TableNextColumn();
        if (s.Price > 0)
        {
            var paid = s.Paid;
            if (ImGui.Checkbox($"##paid{s.Id}", ref paid))
                this.Change(s, new GposeSignupChange { Paid = paid });
        }
        else
        {
            ImGui.TextDisabled("-");
        }
    }

    private void SetStatus(GposeSignup s, GposeSignupChangeStatus status) =>
        this.Change(s, new GposeSignupChange { Status = status });

    private void Change(GposeSignup s, GposeSignupChange change) => this.Run(async () =>
    {
        await this.api.ChangeGposeSignupAsync(s.BoothId, s.Id, change);
        await this.ReloadSignupsAsync(s.BoothId);
    });

    private void Call(GposeSignup s) => this.Run(async () =>
    {
        await this.api.CallGposeSignupAsync(s.BoothId, s.Id);
        await this.ReloadSignupsAsync(s.BoothId);
    });

    private static string BoothLabel(GposeBooth b) =>
        $"{b.Title} ({(b.Status == GposeBoothStatus.Open ? "open" : "closed")})";

    private static string CountsText(GposeBooth b)
    {
        var text = b.Mode == GposeBoothMode.Basic
            ? $"{b.BookedCount} of {b.MaxSignups} places taken"
            : $"{b.BookedCount} booked";
        return b.MaxWaitlist > 0 ? $"{text} - {b.WaitlistCount} of {b.MaxWaitlist} on the waitlist" : text;
    }

    private static string StatusWord(GposeSignupStatus status) => status switch
    {
        GposeSignupStatus.Completed => "Completed",
        GposeSignupStatus.No_show => "No-show",
        GposeSignupStatus.Canceled => "Canceled",
        _ => "Waiting",
    };

    private static Vector4 StatusColor(GposeSignupStatus status) => status switch
    {
        GposeSignupStatus.Completed => Ui.SuccessColor,
        GposeSignupStatus.No_show => Ui.WarnColor,
        _ => Ui.InfoColor,
    };

    private static string Gil(long amount) => $"{amount.ToString("N0", CultureInfo.InvariantCulture)} gil";

    /// <summary>A slot in the operator's own timezone: "Sat 18:00-18:15".</summary>
    private static string SlotText(GposeSignup s)
    {
        if (s.SlotStart is not { } start)
            return "-";
        var from = start.ToLocalTime();
        var to = s.SlotEnd?.ToLocalTime();
        var text = from.ToString("ddd HH:mm", CultureInfo.InvariantCulture);
        return to is { } end ? $"{text}-{end.ToString("HH:mm", CultureInfo.InvariantCulture)}" : text;
    }

    private static string LocalTime(string at) =>
        DateTimeOffset.TryParse(at, CultureInfo.InvariantCulture, DateTimeStyles.AssumeUniversal, out var t)
            ? t.ToLocalTime().ToString("ddd MMM d HH:mm", CultureInfo.InvariantCulture)
            : at;
}
