import React, {useEffect, useState} from 'react';
import moment from 'moment';
import 'moment/locale/fr';
import fr from 'antd/lib/calendar/locale/fr_FR';
import {Badge, Calendar, Input} from 'antd';
import axios from "axios";
import {getBaseUrl} from "../treeFolder";
import {LeftOutlined, RightOutlined, RollbackOutlined} from "@ant-design/icons";

const groupDates = (rawDates,getNb=d=>d.Nb,parseDate=d=>new Date(d.Date).toISOString().slice(0,10).replace(/-/g,"")) => {
    let dates = {};
    rawDates.forEach(d=>{
        let value = {nb:getNb(d),date:parseDate(d)};
        let key = value.date.slice(0,6);
        if(dates[key] == null){
            dates[key] = [value];
        }else{
            dates[key].push(value);
        }
    });
    return dates;
}

const getAllDates = (setDates,url)=> {
    let baseUrl = getBaseUrl();
    axios({
        method:'GET',
        url:baseUrl+url,
    }).then(d=>setDates(groupDates(d.data)));
};

function dateCellRender(value,dates){
    if(dates == null){return;}
    let key = value.format('YYYYMM');
    let fullKey = value.format('YYYYMMDD');
    let days = dates[key];

    let count = days != null ? days.filter(day=>day.date === fullKey).reduce((somme,d)=>somme+d.nb,0) : 0;
    return count > 0 ?(
        <div className="notes-month">
            <Badge count={count} overflowCount={999} style={{ backgroundColor: '#002d4b' }}/>
        </div>
    ):null;
}

function monthCellRender(value,dates) {
    if(dates == null || dates == null){return;}
    let key = parseInt(value.format('YYYYMM'));
    let photos = dates[key];
    return photos != null ? (
        <div className="notes-month">
            <Badge status={"success"} text={`${photos.length} jour(s)`}/>
        </div>
    ) : null;
}

function header(infos,mode,setMode) {
    switch (mode) {
        case 'year':
            let previous = moment(infos.value).subtract(1, 'year');
            let next = moment(infos.value).add(1, 'year');
            previous.navigation=true;
            next.navigation=true;
            return (
                <div className="header-calendar">
                    <button onClick={() => infos.onChange(previous)}><LeftOutlined/></button>
                    <span>{infos.value.year()}</span>
                    {infos.value.year() < moment().year() ?
                        <button onClick={() => infos.onChange(next)}><RightOutlined/></button> :
                        <span style={{width:30+'px',display:'inline-block'}}></span>}
                </div>
            );
        case 'month':
            let previousMonth = moment(infos.value).subtract(1, 'month');
            let nextMonth = moment(infos.value).add(1, 'month');
            previousMonth.navigation=true;
            nextMonth.navigation=true;

            return <div className="header-calendar">
                <button onClick={() => infos.onChange(previousMonth)}><LeftOutlined/></button>
                <span>{infos.value.format("MM/YYYY")}</span>
                <button onClick={() => infos.onChange(nextMonth)}><RightOutlined/></button>
                <button onClick={()=>setMode('year')}><RollbackOutlined /></button>
            </div>;
        default :return "";
    }
}

function onSelect(dates,value,mode,setMode,setUrlFolder,setTitleGallery,getByDate){
    // If action comes from navigation bar, return
    if(value.navigation){
        return;
    }
    if(mode === 'year') {
        setMode("month");
    }else{
        let key = parseInt(value.format('YYYYMM'));
        if(dates[key] == null || !dates[key].some(d=>d.date === value.format('YYYYMMDD'))){return;}

        // Check if photos exist for this date
        setTitleGallery(value.format('DD/MM/YYYY') + " - ");
        // Load gallery with date and url
        setUrlFolder({load:`${getBaseUrl()}/${getByDate}?date=${value.format('YYYYMMDD')}`,tags:`${getBaseUrl()}/tagsByDate/${value.format('YYYYMMDD')}`});
    }
}

export default function MyCalendar({setUrlFolder,setTitleGallery,update,urls}) {
    const [dates,setDates] = useState([]);
    const [originalDates,setOriginalDates] = useState([]);
    const [mode,setMode] = useState('year');
    const { Search } = Input;
    useEffect(()=>{
        setDates([]);
        getAllDates(dates=>{
            setDates(dates);
            setOriginalDates(dates);
        },urls.getAll);
    },[setDates,update,urls]);

    const filter = value => {
        if(value === ""){
            setDates(originalDates)
        }
        // ask server
        axios({
            url:getBaseUrl() + `/filterTagsDate?value=${value}`
        }).then(d=>{
            // Create map
            let map = {};
            d.data.forEach(d=>map[d.slice(0,6)]=true);
            let filteredDates = Object.values(originalDates).flatMap(a=>a).filter(l=>map[l.date.slice(0,6)]);
            setDates(groupDates(filteredDates,d=>d.nb,d=>d.date));
        })
    };

    const filterTree = event => {
        if(event.key === "Enter"){
            filter(event.target.value);
        }
    };

    return (
        <>
        <Search onKeyUp={filterTree} size={"small"} placeholder={"Filtrer par tag"} style={{width:300+'px'}}/>
        <Calendar headerRender={infos=>header(infos,mode,setMode)}
                  dateCellRender={value=>dateCellRender(value,dates)}
                  locale={fr}
                  monthCellRender={value=>monthCellRender(value,dates)} mode={mode}
                  onSelect={value=>onSelect(dates,value,mode,setMode,setUrlFolder,setTitleGallery,urls.getByDate)}/>
                  </>
                  )
}